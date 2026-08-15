package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Benitoow/theia-media/internal/tmdb"
)

// ErrNoMetadataSource is returned when a correction is attempted with no TMDB
// key configured. There is nothing to choose between without one.
var ErrNoMetadataSource = errors.New("library: no metadata source is configured")

// MovieCandidates lists the films a title could refer to.
//
// query is what the user typed. When it is empty the film's own parsed title
// and year are used, which is the search that produced the wrong answer in the
// first place — and is very often enough, because the right film was sitting
// second in a list nobody was shown.
func (s *Service) MovieCandidates(ctx context.Context, id int64, query string) ([]tmdb.Candidate, error) {
	if s.tmdb == nil {
		return nil, ErrNoMetadataSource
	}
	title, year, err := s.store.movieSearchSeed(ctx, id)
	if err != nil {
		return nil, err
	}
	if query != "" {
		// A typed query is a correction of the title itself, so the year parsed
		// out of the filename is no longer evidence of anything.
		title, year = query, 0
	}
	return s.tmdb.Candidates(ctx, title, year)
}

// SetMovieMatch pins a film to the TMDB record somebody chose.
//
// The record is fetched in full and written immediately, rather than left for
// the next scan: this is a correction made while looking at the wrong poster,
// and the right one should replace it before the page is closed.
func (s *Service) SetMovieMatch(ctx context.Context, profileID, id int64, tmdbID int) (Movie, error) {
	if s.tmdb == nil {
		return Movie{}, ErrNoMetadataSource
	}
	if tmdbID <= 0 {
		return Movie{}, fmt.Errorf("library: %d is not a TMDB id", tmdbID)
	}
	if _, _, err := s.store.movieSearchSeed(ctx, id); err != nil {
		return Movie{}, err
	}

	film, err := s.tmdb.Details(ctx, tmdbID)
	if err != nil {
		return Movie{}, err
	}
	if err := s.store.pinMovieMatch(ctx, id, film, time.Now()); err != nil {
		return Movie{}, err
	}

	// Two files of the same film become one card, exactly as they would have if
	// the automatic match had been right to begin with.
	if _, err := s.store.Consolidate(ctx); err != nil {
		s.log.Warn("could not consolidate after a match correction", "id", id, "error", err)
	}
	return s.store.Get(ctx, profileID, id)
}

// ClearMovieMatch hands a film back to the automatic matcher.
func (s *Service) ClearMovieMatch(ctx context.Context, id int64) error {
	return s.store.clearMovieMatch(ctx, id)
}

// SeriesCandidates lists the shows a series could be.
func (s *Service) SeriesCandidates(ctx context.Context, id int64, query string) ([]tmdb.Candidate, error) {
	if s.tmdb == nil {
		return nil, ErrNoMetadataSource
	}
	title, year, err := s.store.seriesSearchSeed(ctx, id)
	if err != nil {
		return nil, err
	}
	if query != "" {
		title, year = query, 0
	}
	return s.tmdb.TVCandidates(ctx, title, year)
}

// SetSeriesMatch pins a series to a chosen show and re-reads it entirely.
//
// A series carries far more than its own record: every season and every episode
// title came from the show it was matched to. Correcting the series without
// them would leave a page headed by the right show and filled with another
// one's episodes, which is worse than the original mistake because it looks
// deliberate. refreshSeries is the same cascade a scan runs, reused whole.
func (s *Service) SetSeriesMatch(ctx context.Context, profileID, id int64, tmdbID int) (Series, error) {
	if s.tmdb == nil {
		return Series{}, ErrNoMetadataSource
	}
	if tmdbID <= 0 {
		return Series{}, fmt.Errorf("library: %d is not a TMDB id", tmdbID)
	}
	title, year, err := s.store.seriesSearchSeed(ctx, id)
	if err != nil {
		return Series{}, err
	}
	if err := s.store.pinSeriesMatch(ctx, id, tmdbID); err != nil {
		return Series{}, err
	}

	// A discarded report: the counters belong to a scan, and this is one row.
	// The lookup failures it would have recorded surface as the error below.
	var report ScanReport
	s.refreshSeries(ctx, staleSeriesCandidate{
		ID: id, Title: title, Year: year, TMDBID: tmdbID,
	}, &report)

	if _, err := s.store.ConsolidateSeries(ctx); err != nil {
		s.log.Warn("could not consolidate after a match correction", "id", id, "error", err)
	}
	return s.store.GetSeries(ctx, profileID, id)
}

// ClearSeriesMatch hands a series back to the automatic matcher.
func (s *Service) ClearSeriesMatch(ctx context.Context, id int64) error {
	return s.store.clearSeriesMatch(ctx, id)
}

// movieSearchSeed reads what the automatic matcher would have searched for.
func (s *Store) movieSearchSeed(ctx context.Context, id int64) (string, int, error) {
	var (
		title string
		year  sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT title, year FROM movies WHERE id = ?`, id).Scan(&title, &year)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, ErrNoSuchMovie
	}
	if err != nil {
		return "", 0, fmt.Errorf("reading film %d: %w", id, err)
	}
	return title, int(year.Int64), nil
}

func (s *Store) seriesSearchSeed(ctx context.Context, id int64) (string, int, error) {
	var (
		title string
		year  sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT title, year FROM series WHERE id = ?`, id).Scan(&title, &year)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, ErrNoSuchSeries
	}
	if err != nil {
		return "", 0, fmt.Errorf("reading series %d: %w", id, err)
	}
	return title, int(year.Int64), nil
}

// pinMovieMatch writes the chosen record and marks the identity settled.
func (s *Store) pinMovieMatch(ctx context.Context, id int64, film *tmdb.Film, now time.Time) error {
	if err := s.SaveMetadata(ctx, id, film, now); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE movies SET tmdb_locked = 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("pinning the match for film %d: %w", id, err)
	}
	return nil
}

// clearMovieMatch unpins a film and puts it back in the queue, so the next pass
// searches for it again from its filename.
func (s *Store) clearMovieMatch(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE movies SET tmdb_locked = 0, metadata_status = ?, metadata_fetched_at = 0 WHERE id = ?`,
		statusPending, id)
	if err != nil {
		return fmt.Errorf("clearing the match for film %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchMovie
	}
	return nil
}

func (s *Store) pinSeriesMatch(ctx context.Context, id int64, tmdbID int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE series SET tmdb_id = ?, tmdb_locked = 1 WHERE id = ?`, tmdbID, id)
	if err != nil {
		return fmt.Errorf("pinning the match for series %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchSeries
	}
	return nil
}

// clearSeriesMatch unpins a series. The seasons keep their own TMDB ids until
// the next pass overwrites them, which it will: refreshSeries re-reads every
// local season from whichever show the series resolves to.
func (s *Store) clearSeriesMatch(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE series
		SET tmdb_locked = 0, tmdb_id = NULL, metadata_status = ?, metadata_fetched_at = 0
		WHERE id = ?`, statusPending, id)
	if err != nil {
		return fmt.Errorf("clearing the match for series %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchSeries
	}
	return nil
}
