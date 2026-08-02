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

const defaultSeriesMetadataBatch = 25

type staleSeriesCandidate struct {
	ID     int64
	Title  string
	Year   int
	TMDBID int
}

func (s *Store) StaleSeriesMetadata(ctx context.Context, now time.Time, limit int) ([]staleSeriesCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT se.id, se.title, se.year, se.tmdb_id
		FROM series se
		WHERE se.metadata_status = ?
		   OR (se.metadata_status = ? AND se.metadata_fetched_at < ?)
		   OR (se.metadata_status IN (?, ?) AND se.metadata_fetched_at < ?)
		   OR EXISTS (
			SELECT 1 FROM seasons s
			WHERE s.series_id = se.id AND (
				s.metadata_status = ?
				OR (s.metadata_status = ? AND s.metadata_fetched_at < ?)
				OR (s.metadata_status IN (?, ?) AND s.metadata_fetched_at < ?)
			)
		   )
		   OR EXISTS (
			SELECT 1 FROM episodes e
			JOIN seasons s ON s.id = e.season_id
			WHERE s.series_id = se.id AND (
				e.metadata_status = ?
				OR (e.metadata_status = ? AND e.metadata_fetched_at < ?)
				OR (e.metadata_status IN (?, ?) AND e.metadata_fetched_at < ?)
			)
		   )
		ORDER BY se.metadata_fetched_at, se.id
		LIMIT ?`,
		statusPending,
		statusOK, now.Add(-metadataLifetime).Unix(),
		statusNotFound, statusError, now.Add(-notFoundLifetime).Unix(),
		statusPending,
		statusOK, now.Add(-metadataLifetime).Unix(),
		statusNotFound, statusError, now.Add(-notFoundLifetime).Unix(),
		statusPending,
		statusOK, now.Add(-metadataLifetime).Unix(),
		statusNotFound, statusError, now.Add(-notFoundLifetime).Unix(),
		limit)
	if err != nil {
		return nil, fmt.Errorf("finding series that need metadata: %w", err)
	}
	defer rows.Close()

	var candidates []staleSeriesCandidate
	for rows.Next() {
		var candidate staleSeriesCandidate
		var year, tmdbID sql.NullInt64
		if err := rows.Scan(&candidate.ID, &candidate.Title, &year, &tmdbID); err != nil {
			return nil, fmt.Errorf("finding series that need metadata: %w", err)
		}
		candidate.Year = int(year.Int64)
		candidate.TMDBID = int(tmdbID.Int64)
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (s *Store) SaveSeriesMetadata(ctx context.Context, id int64, series *tmdb.TVSeries, now time.Time) error {
	genres, err := json.Marshal(series.Genres)
	if err != nil {
		return fmt.Errorf("encoding series genres: %w", err)
	}
	credits := make([]Credit, 0, len(series.Cast))
	for _, person := range series.Cast {
		credits = append(credits, Credit{Name: person.Name, Character: person.Character})
	}
	castJSON, err := json.Marshal(credits)
	if err != nil {
		return fmt.Errorf("encoding series cast: %w", err)
	}
	creatorsJSON, err := json.Marshal(series.Creators)
	if err != nil {
		return fmt.Errorf("encoding series creators: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE series SET
			tmdb_id = ?, tmdb_name = ?, original_name = ?, overview = ?,
			first_air_date = ?, poster_path = ?, backdrop_path = ?, vote_average = ?,
			genres_json = ?, cast_json = ?, creators_json = ?,
			metadata_status = ?, metadata_fetched_at = ?, updated_at = ?
		WHERE id = ?`,
		series.TMDBID, series.Name, series.OriginalName, series.Overview,
		series.FirstAirDate, series.PosterPath, series.BackdropPath, series.VoteAverage,
		string(genres), string(castJSON), string(creatorsJSON),
		statusOK, now.Unix(), now.Unix(), id)
	if err != nil {
		return fmt.Errorf("saving metadata for series %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchSeries
	}
	return nil
}

func (s *Store) MarkSeriesMetadataOutcome(ctx context.Context, id int64, status string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE series SET metadata_status = ?, metadata_fetched_at = ? WHERE id = ?`,
		status, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("recording metadata outcome for series %d: %w", id, err)
	}
	return nil
}

func (s *Store) LocalSeasonNumbers(ctx context.Context, seriesID int64) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT season_number FROM seasons WHERE series_id = ? ORDER BY season_number`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("reading local seasons for series %d: %w", seriesID, err)
	}
	defer rows.Close()
	var numbers []int
	for rows.Next() {
		var number int
		if err := rows.Scan(&number); err != nil {
			return nil, err
		}
		numbers = append(numbers, number)
	}
	return numbers, rows.Err()
}

// SaveSeasonMetadata updates only rows backed by local files. TMDB may know
// twelve seasons; Theia does not create eleven empty seasons just to look busy.
func (s *Store) SaveSeasonMetadata(ctx context.Context, seriesID int64, season *tmdb.TVSeason, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("saving season metadata: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE seasons SET
			tmdb_id = ?, name = ?, overview = ?, air_date = ?, poster_path = ?,
			episode_count = ?, metadata_status = ?, metadata_fetched_at = ?, updated_at = ?
		WHERE series_id = ? AND season_number = ?`,
		season.TMDBID, season.Name, season.Overview, season.AirDate, season.PosterPath,
		season.EpisodeCount, statusOK, now.Unix(), now.Unix(), seriesID, season.Number)
	if err != nil {
		return fmt.Errorf("saving season %d metadata: %w", season.Number, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchSeason
	}

	seen := make(map[int]bool, len(season.Episodes))
	for _, episode := range season.Episodes {
		seen[episode.Number] = true
		if _, err := tx.ExecContext(ctx, `
			UPDATE episodes SET
				tmdb_id = ?, name = ?, overview = ?, air_date = ?, still_path = ?,
				runtime_minutes = ?, vote_average = ?, metadata_status = ?, metadata_fetched_at = ?
			WHERE season_id = (
				SELECT id FROM seasons WHERE series_id = ? AND season_number = ?
			) AND episode_number = ?`,
			episode.TMDBID, episode.Name, episode.Overview, episode.AirDate,
			episode.StillPath, nullPositiveInt(episode.RuntimeMinutes), episode.VoteAverage,
			statusOK, now.Unix(), seriesID, season.Number, episode.Number); err != nil {
			return fmt.Errorf("saving episode %d metadata: %w", episode.Number, err)
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.episode_number
		FROM episodes e JOIN seasons s ON s.id = e.season_id
		WHERE s.series_id = ? AND s.season_number = ?`, seriesID, season.Number)
	if err != nil {
		return fmt.Errorf("marking absent episode metadata: %w", err)
	}
	var missing []int64
	for rows.Next() {
		var id int64
		var number int
		if err := rows.Scan(&id, &number); err != nil {
			rows.Close()
			return err
		}
		if !seen[number] {
			missing = append(missing, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range missing {
		if _, err := tx.ExecContext(ctx, `
			UPDATE episodes SET metadata_status = ?, metadata_fetched_at = ? WHERE id = ?`,
			statusNotFound, now.Unix(), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) MarkSeasonMetadataOutcome(ctx context.Context, seriesID int64, number int, status string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `
		UPDATE seasons SET metadata_status = ?, metadata_fetched_at = ?
		WHERE series_id = ? AND season_number = ?`, status, now.Unix(), seriesID, number); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE episodes SET metadata_status = ?, metadata_fetched_at = ?
		WHERE season_id = (
			SELECT id FROM seasons WHERE series_id = ? AND season_number = ?
		)`, status, now.Unix(), seriesID, number); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) enrichSeries(ctx context.Context, report *ScanReport) {
	if s.tmdb == nil {
		return
	}
	candidates, err := s.store.StaleSeriesMetadata(ctx, time.Now(), defaultSeriesMetadataBatch)
	if err != nil {
		s.log.Error("could not find series needing metadata", "error", err)
		report.Problems = append(report.Problems,
			scanner.Problem{Kind: scanner.KindMetadataUnavailable})
		return
	}
	if len(candidates) == 0 {
		return
	}
	s.log.Info("fetching series metadata", "series", len(candidates))

	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return
		}
		var series *tmdb.TVSeries
		if candidate.TMDBID > 0 {
			series, err = s.tmdb.TVDetails(ctx, candidate.TMDBID)
		} else {
			series, err = s.tmdb.LookupTV(ctx, candidate.Title, candidate.Year)
		}
		switch {
		case err == nil:
			if err := s.store.SaveSeriesMetadata(ctx, candidate.ID, series, time.Now()); err != nil {
				s.log.Warn("could not save series metadata", "title", candidate.Title, "error", err)
				report.Problems = append(report.Problems,
					scanner.Problem{Kind: scanner.KindSaveFailed})
				continue
			}
			report.Enriched++

		case errors.Is(err, tmdb.ErrNotFound):
			_ = s.store.MarkSeriesMetadataOutcome(ctx, candidate.ID, statusNotFound, time.Now())
			report.NotFound++
			continue

		case errors.Is(err, tmdb.ErrUnauthorized):
			s.log.Error("TMDB rejected the API key, stopping series metadata lookups")
			report.Problems = append(report.Problems,
				scanner.Problem{Kind: scanner.KindMetadataKeyRejected})
			return

		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return

		default:
			s.log.Warn("series metadata lookup failed", "title", candidate.Title, "error", err)
			_ = s.store.MarkSeriesMetadataOutcome(ctx, candidate.ID, statusError, time.Now())
			report.MetadataErrors++
			continue
		}

		seasonNumbers, err := s.store.LocalSeasonNumbers(ctx, candidate.ID)
		if err != nil {
			s.log.Warn("could not read local seasons", "series_id", candidate.ID, "error", err)
			report.MetadataErrors++
			continue
		}
		for _, number := range seasonNumbers {
			season, err := s.tmdb.TVSeasonDetails(ctx, series.TMDBID, number)
			switch {
			case err == nil:
				if err := s.store.SaveSeasonMetadata(ctx, candidate.ID, season, time.Now()); err != nil {
					s.log.Warn("could not save season metadata", "series_id", candidate.ID,
						"season", number, "error", err)
					report.MetadataErrors++
				}
			case errors.Is(err, tmdb.ErrNotFound):
				_ = s.store.MarkSeasonMetadataOutcome(ctx, candidate.ID, number, statusNotFound, time.Now())
				report.NotFound++
			case errors.Is(err, tmdb.ErrUnauthorized):
				report.Problems = append(report.Problems,
					scanner.Problem{Kind: scanner.KindMetadataKeyRejected})
				return
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				return
			default:
				_ = s.store.MarkSeasonMetadataOutcome(ctx, candidate.ID, number, statusError, time.Now())
				report.MetadataErrors++
			}
		}
	}
}
