package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Benitoow/theia-media/internal/scanner"
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
	TMDBID           int      `json:"tmdb_id,omitempty"`
	Title            string   `json:"tmdb_title,omitempty"`
	OriginalTitle    string   `json:"original_title,omitempty"`
	OriginalLanguage string   `json:"original_language,omitempty"`
	Tagline          string   `json:"tagline,omitempty"`
	Overview         string   `json:"overview,omitempty"`
	ReleaseDate      string   `json:"release_date,omitempty"`
	PosterPath       string   `json:"poster_path,omitempty"`
	BackdropPath     string   `json:"backdrop_path,omitempty"`
	Runtime          int      `json:"runtime_minutes,omitempty"`
	VoteAverage      float64  `json:"vote_average,omitempty"`
	Director         string   `json:"director,omitempty"`
	Genres           []string `json:"genres,omitempty"`
	Cast             []Credit `json:"cast,omitempty"`

	// Crew is the named crew beyond the director. Each entry carries a role
	// code; the interface owns the word for it (decision 25).
	Crew []Credit `json:"crew,omitempty"`

	// Certification is an age rating as its board wrote it, and the country
	// that board sits in. Both or neither.
	Certification        string `json:"certification,omitempty"`
	CertificationCountry string `json:"certification_country,omitempty"`

	// Collection is the saga this film is one part of, when TMDB says it is
	// part of one. Which other parts the household owns is a library question,
	// answered separately.
	Collection *Collection `json:"collection,omitempty"`

	Status    string    `json:"status"`
	FetchedAt time.Time `json:"fetched_at,omitempty"`
}

// Collection is the TMDB grouping a film belongs to.
type Collection struct {
	TMDBID     int    `json:"tmdb_id"`
	Name       string `json:"name"`
	PosterPath string `json:"poster_path,omitempty"`
}

// Credit is one credited name. Character is set for the cast, Role for the
// crew, and ProfilePath is the portrait when TMDB holds one.
type Credit struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	Role        string `json:"role,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
}

// currentMetadataVersion is the field set the code writes today.
//
// A row stamped with less than this has columns the code now reads and TMDB
// already answered with -- so it is stale regardless of its age, and the next
// scan refetches it in the ordinary batches. Bumping this is how a new TMDB
// field reaches a library that was scanned before the field existed.
//
//	1  the original 0002 columns
//	2  tagline, original title and language, certificate, collection, crew,
//	   cast portraits (migration 0013)
const currentMetadataVersion = 2

// staleCandidate is a film whose metadata needs looking up.
type staleCandidate struct {
	ID    int64
	Title string
	Year  int

	// TMDBID and Locked are how a corrected match survives a refresh. Locked
	// means somebody chose this film by hand, so the refresh reads that record
	// by id rather than searching the title again and finding the same wrong
	// answer it found the first time.
	TMDBID int
	Locked bool
}

// StaleMetadata returns films whose metadata has never been fetched or has
// aged out, oldest first so that a library larger than one batch still makes
// progress on every scan.
func (s *Store) StaleMetadata(ctx context.Context, now time.Time, limit int) ([]staleCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, year, tmdb_id, tmdb_locked
		FROM movies
		WHERE metadata_status = ?
		   OR (metadata_status = ? AND metadata_fetched_at < ?)
		   OR (metadata_status IN (?, ?) AND metadata_fetched_at < ?)
		   OR (metadata_status = ? AND metadata_version < ?)
		ORDER BY metadata_fetched_at, id
		LIMIT ?`,
		statusPending,
		statusOK, now.Add(-metadataLifetime).Unix(),
		statusNotFound, statusError, now.Add(-notFoundLifetime).Unix(),
		statusOK, currentMetadataVersion,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("finding films that need metadata: %w", err)
	}
	defer rows.Close()

	var out []staleCandidate
	for rows.Next() {
		var (
			c            staleCandidate
			year, tmdbID sql.NullInt64
			locked       int
		)
		if err := rows.Scan(&c.ID, &c.Title, &year, &tmdbID, &locked); err != nil {
			return nil, fmt.Errorf("finding films that need metadata: %w", err)
		}
		if year.Valid {
			c.Year = int(year.Int64)
		}
		if tmdbID.Valid {
			c.TMDBID = int(tmdbID.Int64)
		}
		c.Locked = locked == 1
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

	castJSON, err := json.Marshal(creditsFrom(film.Cast))
	if err != nil {
		return fmt.Errorf("encoding cast: %w", err)
	}
	crewJSON, err := json.Marshal(creditsFrom(film.Crew))
	if err != nil {
		return fmt.Errorf("encoding crew: %w", err)
	}

	var collectionID sql.NullInt64
	var collectionName, collectionPoster string
	if film.Collection != nil {
		collectionID = sql.NullInt64{Int64: int64(film.Collection.TMDBID), Valid: true}
		collectionName = film.Collection.Name
		collectionPoster = film.Collection.PosterPath
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE movies SET
			tmdb_id                = ?,
			tmdb_title             = ?,
			original_title         = ?,
			original_language      = ?,
			tagline                = ?,
			overview               = ?,
			release_date           = ?,
			poster_path            = ?,
			backdrop_path          = ?,
			runtime_minutes        = ?,
			vote_average           = ?,
			director               = ?,
			genres_json            = ?,
			cast_json              = ?,
			crew_json              = ?,
			certification          = ?,
			certification_country  = ?,
			collection_id          = ?,
			collection_name        = ?,
			collection_poster_path = ?,
			metadata_status        = ?,
			metadata_fetched_at    = ?,
			metadata_version       = ?,
			updated_at             = ?
		WHERE id = ?`,
		film.TMDBID, film.Title, film.OriginalTitle, film.OriginalLanguage,
		film.Tagline, film.Overview, film.ReleaseDate,
		film.PosterPath, film.BackdropPath, film.Runtime, film.VoteAverage,
		film.Director, string(genres), string(castJSON), string(crewJSON),
		film.Certification, film.CertificationCountry,
		collectionID, collectionName, collectionPoster,
		statusOK, now.Unix(), currentMetadataVersion, now.Unix(), id,
	)
	if err != nil {
		return fmt.Errorf("saving metadata for film %d: %w", id, err)
	}
	return nil
}

// creditsFrom converts TMDB people into stored credits. It is shared by the
// film and series paths so that a portrait cannot be kept in one and dropped in
// the other.
func creditsFrom(people []tmdb.Person) []Credit {
	credits := make([]Credit, 0, len(people))
	for _, person := range people {
		credits = append(credits, Credit{
			Name:        person.Name,
			Character:   person.Character,
			Role:        person.Role,
			ProfilePath: person.ProfilePath,
		})
	}
	return credits
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
		s.log.Error("could not find films needing metadata", "error", err)
		report.Problems = append(report.Problems,
			scanner.Problem{Kind: scanner.KindMetadataUnavailable})
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

		// A film somebody matched by hand is refreshed by id. Searching its
		// title again would rediscover whatever wrong film that title leads to,
		// and quietly undo the correction ninety days after it was made.
		var (
			film *tmdb.Film
			err  error
		)
		if candidate.Locked && candidate.TMDBID > 0 {
			film, err = s.tmdb.Details(ctx, candidate.TMDBID)
		} else {
			film, err = s.tmdb.Lookup(ctx, candidate.Title, candidate.Year)
		}
		switch {
		case err == nil:
			if err := s.store.SaveMetadata(ctx, candidate.ID, film, time.Now()); err != nil {
				s.log.Warn("could not save metadata", "title", candidate.Title, "error", err)
				report.Problems = append(report.Problems,
					scanner.Problem{Kind: scanner.KindSaveFailed})
				continue
			}
			report.Enriched++

		case errors.Is(err, tmdb.ErrNotFound):
			// Ordinary, and not a problem: the film keeps the title its
			// filename gave it. Counted, never reported as something wrong.
			if err := s.store.MarkMetadataOutcome(ctx, candidate.ID, statusNotFound, time.Now()); err != nil {
				s.log.Warn("could not record a metadata miss", "title", candidate.Title, "error", err)
			}
			report.NotFound++

		case errors.Is(err, tmdb.ErrUnauthorized):
			s.log.Error("TMDB rejected the API key, stopping metadata lookups")
			report.Problems = append(report.Problems,
				scanner.Problem{Kind: scanner.KindMetadataKeyRejected})
			return

		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return

		default:
			s.log.Warn("metadata lookup failed", "title", candidate.Title, "error", err)
			if err := s.store.MarkMetadataOutcome(ctx, candidate.ID, statusError, time.Now()); err != nil {
				s.log.Warn("could not record a metadata failure", "title", candidate.Title, "error", err)
			}
			report.MetadataErrors++
		}
	}
}

// metadataRow holds the metadata columns of one film row as SQLite returns
// them, before the NULLs have been decided.
//
// It exists so that adding a TMDB field is two edits -- the column list and the
// two methods below -- rather than another parameter threaded through a
// positional function that already took fourteen.
type metadataRow struct {
	tmdbID       sql.NullInt64
	tmdbTitle    sql.NullString
	originalName sql.NullString
	originalLang sql.NullString
	tagline      sql.NullString
	overview     sql.NullString
	releaseDate  sql.NullString
	posterPath   sql.NullString
	backdropPath sql.NullString
	director     sql.NullString
	genresJSON   sql.NullString
	castJSON     sql.NullString
	crewJSON     sql.NullString

	certification        sql.NullString
	certificationCountry sql.NullString

	collectionID     sql.NullInt64
	collectionName   sql.NullString
	collectionPoster sql.NullString

	runtime   sql.NullInt64
	vote      sql.NullFloat64
	status    string
	fetchedAt int64
}

// targets returns the scan destinations, in the order metadataColumns lists
// them. The two must be read side by side; a mismatch shifts every field by one
// and produces rows that look plausible.
func (r *metadataRow) targets() []any {
	return []any{
		&r.tmdbID, &r.tmdbTitle, &r.originalName, &r.originalLang, &r.tagline,
		&r.overview, &r.releaseDate, &r.posterPath, &r.backdropPath,
		&r.director, &r.genresJSON, &r.castJSON, &r.crewJSON,
		&r.certification, &r.certificationCountry,
		&r.collectionID, &r.collectionName, &r.collectionPoster,
		&r.runtime, &r.vote, &r.status, &r.fetchedAt,
	}
}

// fill copies the row onto a film.
func (r *metadataRow) fill(m *Movie) {
	m.Metadata.Status = r.status
	if r.fetchedAt > 0 {
		m.Metadata.FetchedAt = time.Unix(r.fetchedAt, 0).UTC()
	}
	if r.tmdbID.Valid {
		m.Metadata.TMDBID = int(r.tmdbID.Int64)
	}
	m.Metadata.Title = r.tmdbTitle.String
	m.Metadata.OriginalTitle = r.originalName.String
	m.Metadata.OriginalLanguage = r.originalLang.String
	m.Metadata.Tagline = r.tagline.String
	m.Metadata.Overview = r.overview.String
	m.Metadata.ReleaseDate = r.releaseDate.String
	m.Metadata.PosterPath = r.posterPath.String
	m.Metadata.BackdropPath = r.backdropPath.String
	m.Metadata.Director = r.director.String
	m.Metadata.Certification = r.certification.String
	m.Metadata.CertificationCountry = r.certificationCountry.String
	if r.runtime.Valid {
		m.Metadata.Runtime = int(r.runtime.Int64)
	}
	if r.vote.Valid {
		m.Metadata.VoteAverage = r.vote.Float64
	}
	if r.genresJSON.Valid && r.genresJSON.String != "" {
		_ = json.Unmarshal([]byte(r.genresJSON.String), &m.Metadata.Genres)
	}
	if r.castJSON.Valid && r.castJSON.String != "" {
		_ = json.Unmarshal([]byte(r.castJSON.String), &m.Metadata.Cast)
	}
	if r.crewJSON.Valid && r.crewJSON.String != "" {
		_ = json.Unmarshal([]byte(r.crewJSON.String), &m.Metadata.Crew)
	}
	// A collection with no id is not a collection. The name alone could not be
	// used to find anything anyway.
	if r.collectionID.Valid && r.collectionID.Int64 > 0 {
		m.Metadata.Collection = &Collection{
			TMDBID:     int(r.collectionID.Int64),
			Name:       r.collectionName.String,
			PosterPath: r.collectionPoster.String,
		}
	}
}
