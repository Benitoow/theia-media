package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type storedFileIdentity struct {
	ID        int64
	MovieID   int64
	FileName  string
	SizeBytes int64
	Modified  int64
	Primary   bool
}

func (s *Store) upsertFile(ctx context.Context, m Movie, generation int64) (upsertResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Classification can change after a rename or parser improvement. Keep one
	// physical path in exactly one media family.
	if err := removeEpisodePathTx(ctx, tx, m.Path); err != nil {
		return upsertResult{}, fmt.Errorf("reclassifying %s as a film: %w", m.Path, err)
	}

	existing, err := fileByPathTx(ctx, tx, m.Path)
	switch {
	case err == nil:
		if err := refreshKnownFileTx(ctx, tx, existing, m, generation); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
		}
		if err := tx.Commit(); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
		}
		return upsertResult{}, nil
	case !errors.Is(err, sql.ErrNoRows):
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	}

	// A unique unseen file with the same size and modification time is a move or
	// rename. This preserves both the stable file id and the film's progress.
	// Ambiguous matches deliberately fall through to ordinary association.
	if moved, ok, err := moveCandidateTx(ctx, tx, m, generation); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	} else if ok {
		if err := refreshMovedFileTx(ctx, tx, moved, m, generation); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
		}
		if err := tx.Commit(); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
		}
		return upsertResult{}, nil
	}

	movieID, found, err := associationCandidateTx(ctx, tx, m)
	if err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	}
	primary := 0
	if !found {
		movieID, err = insertMovieTx(ctx, tx, m, generation)
		if err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
		}
		primary = 1
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO movie_files (
			movie_id, path, file_name, size_bytes, modified_at,
			first_seen_scan, last_seen_scan, is_primary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		movieID, m.Path, m.FileName, m.SizeBytes, m.ModifiedAt.Unix(),
		generation, generation, primary); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET last_seen_scan = ?, updated_at = ? WHERE id = ?`,
		generation, time.Now().Unix(), movieID); err != nil {
		return upsertResult{}, fmt.Errorf("refreshing film %d: %w", movieID, err)
	}
	if primary == 1 {
		if err := mirrorPrimaryTx(ctx, tx, movieID); err != nil {
			return upsertResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	}
	return upsertResult{inserted: true}, nil
}

func fileByPathTx(ctx context.Context, tx *sql.Tx, path string) (storedFileIdentity, error) {
	var file storedFileIdentity
	var primary int
	err := tx.QueryRowContext(ctx, `
		SELECT id, movie_id, file_name, size_bytes, modified_at, is_primary
		FROM movie_files WHERE path = ?`, path,
	).Scan(&file.ID, &file.MovieID, &file.FileName, &file.SizeBytes, &file.Modified, &primary)
	file.Primary = primary != 0
	return file, err
}

func refreshKnownFileTx(ctx context.Context, tx *sql.Tx, existing storedFileIdentity,
	m Movie, generation int64,
) error {
	changed := existing.SizeBytes != m.SizeBytes || existing.Modified != m.ModifiedAt.Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE movie_files SET
			file_name = ?, size_bytes = ?, modified_at = ?, last_seen_scan = ?
		WHERE id = ?`,
		m.FileName, m.SizeBytes, m.ModifiedAt.Unix(), generation, existing.ID); err != nil {
		return err
	}
	if changed {
		if err := invalidateFileMediaTx(ctx, tx, existing.ID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET last_seen_scan = ?, updated_at = ? WHERE id = ?`,
		generation, time.Now().Unix(), existing.MovieID); err != nil {
		return err
	}
	if existing.Primary {
		return mirrorPrimaryTx(ctx, tx, existing.MovieID)
	}
	return nil
}

func moveCandidateTx(ctx context.Context, tx *sql.Tx, m Movie, generation int64) (storedFileIdentity, bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, movie_id, file_name, size_bytes, modified_at, is_primary
		FROM movie_files
		WHERE size_bytes = ? AND modified_at = ? AND last_seen_scan < ?
		ORDER BY id
		LIMIT 2`, m.SizeBytes, m.ModifiedAt.Unix(), generation)
	if err != nil {
		return storedFileIdentity{}, false, err
	}
	defer rows.Close()

	var candidates []storedFileIdentity
	for rows.Next() {
		var candidate storedFileIdentity
		var primary int
		if err := rows.Scan(&candidate.ID, &candidate.MovieID, &candidate.FileName,
			&candidate.SizeBytes, &candidate.Modified, &primary); err != nil {
			return storedFileIdentity{}, false, err
		}
		candidate.Primary = primary != 0
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil || len(candidates) != 1 {
		return storedFileIdentity{}, false, err
	}

	var title string
	var year sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT title, year FROM movies WHERE id = ?`, candidates[0].MovieID,
	).Scan(&title, &year); err != nil {
		return storedFileIdentity{}, false, err
	}
	var fileCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM movie_files WHERE movie_id = ?`, candidates[0].MovieID,
	).Scan(&fileCount); err != nil {
		return storedFileIdentity{}, false, err
	}

	oldYear := 0
	if year.Valid {
		oldYear = int(year.Int64)
	}
	// A multi-file film may lose one variant and gain another only when the
	// parsed identity still agrees. Otherwise treating the byte match as a move
	// could silently steal a file from the wrong film.
	if fileCount > 1 && !sameParsedIdentity(title, oldYear, m.Title, m.Year) {
		return storedFileIdentity{}, false, nil
	}
	return candidates[0], true, nil
}

func refreshMovedFileTx(ctx context.Context, tx *sql.Tx, moved storedFileIdentity,
	m Movie, generation int64,
) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE movie_files SET
			path = ?, file_name = ?, size_bytes = ?, modified_at = ?, last_seen_scan = ?
		WHERE id = ?`,
		m.Path, m.FileName, m.SizeBytes, m.ModifiedAt.Unix(), generation, moved.ID); err != nil {
		return err
	}

	var title string
	var year sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT title, year FROM movies WHERE id = ?`, moved.MovieID,
	).Scan(&title, &year); err != nil {
		return err
	}
	oldYear := 0
	if year.Valid {
		oldYear = int(year.Int64)
	}
	if !sameParsedIdentity(title, oldYear, m.Title, m.Year) {
		var fileCount int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM movie_files WHERE movie_id = ?`, moved.MovieID,
		).Scan(&fileCount); err != nil {
			return err
		}
		if fileCount == 1 {
			if err := updateMovieIdentityTx(ctx, tx, moved.MovieID, m.Title, m.Year); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET last_seen_scan = ?, updated_at = ? WHERE id = ?`,
		generation, time.Now().Unix(), moved.MovieID); err != nil {
		return err
	}
	if moved.Primary {
		return mirrorPrimaryTx(ctx, tx, moved.MovieID)
	}
	return nil
}

func associationCandidateTx(ctx context.Context, tx *sql.Tx, m Movie) (int64, bool, error) {
	if m.Year > 0 {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, title, tmdb_id FROM movies WHERE year = ? ORDER BY id`, m.Year)
		if err != nil {
			return 0, false, err
		}
		defer rows.Close()

		type candidate struct {
			id   int64
			tmdb sql.NullInt64
		}
		var matches []candidate
		for rows.Next() {
			var id int64
			var title string
			var tmdbID sql.NullInt64
			if err := rows.Scan(&id, &title, &tmdbID); err != nil {
				return 0, false, err
			}
			if normalizedTitle(title) == normalizedTitle(m.Title) {
				matches = append(matches, candidate{id: id, tmdb: tmdbID})
			}
		}
		if err := rows.Err(); err != nil || len(matches) == 0 {
			return 0, false, err
		}

		// Conflicting non-zero TMDB ids mean the apparent title/year match is not
		// safe enough to merge automatically.
		seenTMDB := map[int64]bool{}
		for _, match := range matches {
			if match.tmdb.Valid {
				seenTMDB[match.tmdb.Int64] = true
			}
		}
		if len(seenTMDB) > 1 {
			return 0, false, nil
		}
		return matches[0].id, true, nil
	}

	// With no year, only the product's original exact-stem rule is safe. Do not
	// normalize separators or collapse a known-year remake into this film.
	wantStem := strings.TrimSuffix(m.FileName, filepath.Ext(m.FileName))
	rows, err := tx.QueryContext(ctx, `
		SELECT movie_id, file_name FROM movie_files ORDER BY movie_id, id`)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	ids := map[int64]bool{}
	for rows.Next() {
		var movieID int64
		var fileName string
		if err := rows.Scan(&movieID, &fileName); err != nil {
			return 0, false, err
		}
		stem := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		if strings.EqualFold(strings.TrimSpace(stem), strings.TrimSpace(wantStem)) {
			ids[movieID] = true
		}
	}
	if err := rows.Err(); err != nil || len(ids) != 1 {
		return 0, false, err
	}
	for id := range ids {
		return id, true, nil
	}
	return 0, false, nil
}

func insertMovieTx(ctx context.Context, tx *sql.Tx, m Movie, generation int64) (int64, error) {
	now := time.Now().Unix()
	var year any
	if m.Year > 0 {
		year = m.Year
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO movies (
			path, file_name, size_bytes, modified_at, title, year,
			first_seen_scan, last_seen_scan, added_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Path, m.FileName, m.SizeBytes, m.ModifiedAt.Unix(), m.Title, year,
		generation, generation, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func updateMovieIdentityTx(ctx context.Context, tx *sql.Tx, movieID int64, title string, year int) error {
	var storedYear any
	if year > 0 {
		storedYear = year
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE movies SET
			title = ?, year = ?,
			tmdb_id = NULL, tmdb_title = NULL, overview = NULL,
			release_date = NULL, poster_path = NULL, backdrop_path = NULL,
			runtime_minutes = NULL, vote_average = NULL, director = NULL,
			genres_json = NULL, cast_json = NULL,
			metadata_status = 'pending',
			metadata_fetched_at = 0, updated_at = ?
		WHERE id = ?`, title, storedYear, time.Now().Unix(), movieID)
	return err
}

func invalidateFileMediaTx(ctx context.Context, tx *sql.Tx, fileID int64) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE movie_files SET
			media_status = 'pending', media_container = NULL,
			media_duration_seconds = NULL,
			video_stream_index = NULL, video_codec = NULL,
			video_width = NULL, video_height = NULL,
			media_inspected_at = 0
		WHERE id = ?`, fileID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		`DELETE FROM movie_file_audio_tracks WHERE movie_file_id = ?`, fileID)
	return err
}

func (s *Store) deleteFilesNotSeenIn(ctx context.Context, generation int64) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("removing files that have disappeared: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var removed int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM movie_files WHERE last_seen_scan < ?`, generation,
	).Scan(&removed); err != nil {
		return 0, fmt.Errorf("removing files that have disappeared: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM movie_files WHERE last_seen_scan < ?`, generation); err != nil {
		return 0, fmt.Errorf("removing files that have disappeared: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM movies
		WHERE NOT EXISTS (SELECT 1 FROM movie_files WHERE movie_id = movies.id)`); err != nil {
		return 0, fmt.Errorf("removing empty films: %w", err)
	}
	if err := repairPrimariesTx(ctx, tx); err != nil {
		return 0, fmt.Errorf("repairing primary files: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("removing files that have disappeared: %w", err)
	}
	return removed, nil
}

func repairPrimariesTx(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT m.id
		FROM movies m
		WHERE NOT EXISTS (
			SELECT 1 FROM movie_files f WHERE f.movie_id = m.id AND f.is_primary = 1
		)
		  AND EXISTS (SELECT 1 FROM movie_files f WHERE f.movie_id = m.id)
		ORDER BY m.id`)
	if err != nil {
		return err
	}
	var movieIDs []int64
	for rows.Next() {
		var movieID int64
		if err := rows.Scan(&movieID); err != nil {
			rows.Close()
			return err
		}
		movieIDs = append(movieIDs, movieID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, movieID := range movieIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE movie_files SET is_primary = 1
			WHERE id = (SELECT MIN(id) FROM movie_files WHERE movie_id = ?)`, movieID); err != nil {
			return err
		}
		if err := mirrorPrimaryTx(ctx, tx, movieID); err != nil {
			return err
		}
	}
	return nil
}

func mirrorPrimaryTx(ctx context.Context, tx *sql.Tx, movieID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE movies SET
			path = (SELECT path FROM movie_files WHERE movie_id = ? AND is_primary = 1),
			file_name = (SELECT file_name FROM movie_files WHERE movie_id = ? AND is_primary = 1),
			size_bytes = (SELECT size_bytes FROM movie_files WHERE movie_id = ? AND is_primary = 1),
			modified_at = (SELECT modified_at FROM movie_files WHERE movie_id = ? AND is_primary = 1)
		WHERE id = ?`, movieID, movieID, movieID, movieID, movieID)
	return err
}

func normalizedTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

func sameParsedIdentity(aTitle string, aYear int, bTitle string, bYear int) bool {
	return aYear == bYear && normalizedTitle(aTitle) == normalizedTitle(bTitle)
}
