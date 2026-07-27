package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Benitoow/theia-media/internal/tmdb"
)

// Metadata lifetimes.
//
// A TMDB record is not immutable -- synopses get rewritten, posters get
// replaced, a missing runtime gets filled in -- so nothing is cached forever.
// But it changes slowly enough that re-reading it more than a few times a year
// is pure waste, hence ninety days.
//
// A film TMDB did not recognise is retried far sooner. The cause is usually on
// our side: a mangled filename the user is about to rename, or a title parsed
// badly enough that a slightly different guess would land. Making the user wait
// three months to see the poster appear after fixing a filename would feel
// broken.
const (
	metadataLifetime = 90 * 24 * time.Hour
	notFoundLifetime = 7 * 24 * time.Hour
)

// Metadata status values, mirroring the CHECK-free text column in migration
// 0002. Kept as constants so a typo is a compile error rather than a row that
// silently never refreshes.
const (
	statusPending  = "pending"
	statusOK       = "ok"
	statusNotFound = "not_found"
	statusError    = "error"
)

// Metadata is what TMDB told us about a film, as stored.
type Metadata struct {
	TMDBID       int       `json:"tmdb_id,omitempty"`
	Title        string    `json:"tmdb_title,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	ReleaseDate  string    `json:"release_date,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	BackdropPath string    `json:"backdrop_path,omitempty"`
	Runtime      int       `json:"runtime_minutes,omitempty"`
	VoteAverage  float64   `json:"vote_average,omitempty"`
	Director     string    `json:"director,omitempty"`
	Genres       []string  `json:"genres,omitempty"`
	Cast         []Credit  `json:"cast,omitempty"`
	Status       string    `json:"status"`
	FetchedAt    time.Time `json:"fetched_at,omitempty"`
}

// Credit is one name in the cast list.
type Credit struct {
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
}

// staleCandidate is a film whose metadata needs looking up.
type staleCandidate struct {
	ID    int64
	Title string
	Year  int
}

// StaleMetadata returns films whose metadata has never been fetched or has
// aged out, oldest first so that a library larger than one batch still makes
// progress on every scan.
func (s *Store) StaleMetadata(ctx context.Context, now time.Time, limit int) ([]staleCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, year
		FROM movies
		WHERE metadata_status = ?
		   OR (metadata_status = ? AND metadata_fetched_at < ?)
		   OR (metadata_status IN (?, ?) AND metadata_fetched_at < ?)
		ORDER BY metadata_fetched_at, id
		LIMIT ?`,
		statusPending,
		statusOK, now.Add(-metadataLifetime).Unix(),
		statusNotFound, statusError, now.Add(-notFoundLifetime).Unix(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("finding films that need metadata: %w", err)
	}
	defer rows.Close()

	var out []staleCandidate
	for rows.Next() {
		var (
			c    staleCandidate
			year sql.NullInt64
		)
		if err := rows.Scan(&c.ID, &c.Title, &year); err != nil {
			return nil, fmt.Errorf("finding films that need metadata: %w", err)
		}
		if year.Valid {
			c.Year = int(year.Int64)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveMetadata records a successful lookup.
func (s *Store) SaveMetadata(ctx context.Context, id int64, film *tmdb.Film, now time.Time) error {
	genres, err := json.Marshal(film.Genres)
	if err != nil {
		return fmt.Errorf("encoding genres: %w", err)
	}

	credits := make([]Credit, 0, len(film.Cast))
	for _, p := range film.Cast {
		credits = append(credits, Credit{Name: p.Name, Character: p.Character})
	}
	castJSON, err := json.Marshal(credits)
	if err != nil {
		return fmt.Errorf("encoding cast: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE movies SET
			tmdb_id             = ?,
			tmdb_title          = ?,
			overview            = ?,
			release_date        = ?,
			poster_path         = ?,
			backdrop_path       = ?,
			runtime_minutes     = ?,
			vote_average        = ?,
			director            = ?,
			genres_json         = ?,
			cast_json           = ?,
			metadata_status     = ?,
			metadata_fetched_at = ?,
			updated_at          = ?
		WHERE id = ?`,
		film.TMDBID, film.Title, film.Overview, film.ReleaseDate,
		film.PosterPath, film.BackdropPath, film.Runtime, film.VoteAverage,
		film.Director, string(genres), string(castJSON),
		statusOK, now.Unix(), now.Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("saving metadata for film %d: %w", id, err)
	}
	return nil
}

// MarkMetadataOutcome records a lookup that produced no film, so that it is not
// retried on every single scan.
func (s *Store) MarkMetadataOutcome(ctx context.Context, id int64, status string, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE movies SET metadata_status = ?, metadata_fetched_at = ? WHERE id = ?`,
		status, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("recording the metadata outcome for film %d: %w", id, err)
	}
	return nil
}

// enrich looks up metadata for films that need it.
//
// Nothing in here can fail a scan. A film TMDB does not know is marked and
// keeps the title the filename gave it; a lookup that errors is marked to be
// retried later. The one exception is a rejected API key, which stops the pass
// immediately -- every remaining lookup would fail identically, and burning
// through the whole library to say so would be both slow and useless.
func (s *Service) enrich(ctx context.Context, report *ScanReport) {
	if s.tmdb == nil {
		return
	}

	now := time.Now()
	candidates, err := s.store.StaleMetadata(ctx, now, s.metadataBatch)
	if err != nil {
		report.Problems = append(report.Problems, err.Error())
		return
	}
	if len(candidates) == 0 {
		return
	}

	s.log.Info("fetching metadata", "films", len(candidates))

	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return
		}

		film, err := s.tmdb.Lookup(ctx, candidate.Title, candidate.Year)
		switch {
		case err == nil:
			if err := s.store.SaveMetadata(ctx, candidate.ID, film, time.Now()); err != nil {
				report.Problems = append(report.Problems, err.Error())
				continue
			}
			report.Enriched++

		case errors.Is(err, tmdb.ErrNotFound):
			// Ordinary. The film keeps its filename-derived title.
			if err := s.store.MarkMetadataOutcome(ctx, candidate.ID, statusNotFound, time.Now()); err != nil {
				report.Problems = append(report.Problems, err.Error())
			}
			report.NotFound++

		case errors.Is(err, tmdb.ErrUnauthorized):
			s.log.Error("TMDB rejected the API key, stopping metadata lookups")
			report.Problems = append(report.Problems,
				"TMDB rejected the API key; check it in the settings")
			return

		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return

		default:
			s.log.Warn("metadata lookup failed", "title", candidate.Title, "error", err)
			if err := s.store.MarkMetadataOutcome(ctx, candidate.ID, statusError, time.Now()); err != nil {
				report.Problems = append(report.Problems, err.Error())
			}
			report.MetadataErrors++
		}
	}
}

// scanMetadata reads the metadata columns off a row being scanned.
func scanMetadata(m *Movie, tmdbID sql.NullInt64, tmdbTitle, overview, releaseDate,
	posterPath, backdropPath, director, genresJSON, castJSON sql.NullString,
	runtime sql.NullInt64, vote sql.NullFloat64, status string, fetchedAt int64,
) {
	m.Metadata.Status = status
	if fetchedAt > 0 {
		m.Metadata.FetchedAt = time.Unix(fetchedAt, 0).UTC()
	}
	if tmdbID.Valid {
		m.Metadata.TMDBID = int(tmdbID.Int64)
	}
	m.Metadata.Title = tmdbTitle.String
	m.Metadata.Overview = overview.String
	m.Metadata.ReleaseDate = releaseDate.String
	m.Metadata.PosterPath = posterPath.String
	m.Metadata.BackdropPath = backdropPath.String
	m.Metadata.Director = director.String
	if runtime.Valid {
		m.Metadata.Runtime = int(runtime.Int64)
	}
	if vote.Valid {
		m.Metadata.VoteAverage = vote.Float64
	}
	if genresJSON.Valid && genresJSON.String != "" {
		_ = json.Unmarshal([]byte(genresJSON.String), &m.Metadata.Genres)
	}
	if castJSON.Valid && castJSON.String != "" {
		_ = json.Unmarshal([]byte(castJSON.String), &m.Metadata.Cast)
	}
}
