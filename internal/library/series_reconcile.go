package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Benitoow/theia-media/internal/scanner"
)

type storedEpisodeFileIdentity struct {
	ID        int64
	ItemID    int64
	SeasonID  int64
	SeriesID  int64
	Path      string
	SizeBytes int64
	Modified  int64
	Primary   bool
}

// UpsertEpisode reconciles one classified episode file in a single
// transaction. The path is removed from the film branch first, so a parser
// correction cannot leave the same file playable as both a film and an
// episode.
func (s *Store) UpsertEpisode(ctx context.Context, file scanner.File, parsed ParsedEpisode, generation int64) (upsertResult, error) {
	if !parsed.Matched || parsed.Ambiguous || parsed.SeriesTitle == "" || len(parsed.EpisodeNumbers) == 0 {
		return upsertResult{}, fmt.Errorf("saving %s: incomplete episode identity", file.Path)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := removeMoviePathTx(ctx, tx, file.Path); err != nil {
		return upsertResult{}, fmt.Errorf("reclassifying %s as an episode: %w", file.Path, err)
	}
	existing, existingErr := episodeFileByPathTx(ctx, tx, file.Path)
	if existingErr == nil {
		matches, err := storedEpisodeIdentityMatchesTx(ctx, tx, existing.ItemID, parsed)
		if err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
		}
		if matches {
			if err := refreshKnownEpisodeFileTx(ctx, tx, existing, existing.ItemID, file, generation); err != nil {
				return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
			}
			if err := tx.Commit(); err != nil {
				return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
			}
			return upsertResult{}, nil
		}
	} else if !errors.Is(existingErr, sql.ErrNoRows) {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, existingErr)
	}
	itemID, err := ensureEpisodeIdentityTx(ctx, tx, parsed)
	if err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	}

	switch {
	case existingErr == nil:
		if err := refreshKnownEpisodeFileTx(ctx, tx, existing, itemID, file, generation); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
		}
		if err := tx.Commit(); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
		}
		return upsertResult{}, nil
	}

	if moved, ok, err := episodeMoveCandidateTx(ctx, tx, itemID, file, generation); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	} else if ok {
		if err := refreshMovedEpisodeFileTx(ctx, tx, moved, itemID, file, generation); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
		}
		if err := tx.Commit(); err != nil {
			return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
		}
		return upsertResult{}, nil
	}

	primary, err := itemNeedsPrimaryTx(ctx, tx, itemID)
	if err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO episode_files (
			episode_item_id, path, file_name, size_bytes, modified_at,
			first_seen_scan, last_seen_scan, is_primary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		itemID, file.Path, file.Name, file.SizeBytes, file.ModifiedAt.Unix(),
		generation, generation, boolInt(primary)); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	}
	if err := touchEpisodeHierarchyTx(ctx, tx, itemID); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	}
	if err := tx.Commit(); err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", file.Path, err)
	}
	return upsertResult{inserted: true}, nil
}

func storedEpisodeIdentityMatchesTx(ctx context.Context, tx *sql.Tx, itemID int64, parsed ParsedEpisode) (bool, error) {
	var title, key string
	var year sql.NullInt64
	var season int
	err := tx.QueryRowContext(ctx, `
		SELECT se.title, se.year, s.season_number, i.episode_key
		FROM episode_items i
		JOIN seasons s ON s.id = i.season_id
		JOIN series se ON se.id = s.series_id
		WHERE i.id = ?`, itemID).Scan(&title, &year, &season, &key)
	if err != nil {
		return false, err
	}
	storedYear := 0
	if year.Valid {
		storedYear = int(year.Int64)
	}
	return sameParsedIdentity(title, storedYear, parsed.SeriesTitle, parsed.SeriesYear) &&
		season == parsed.SeasonNumber && key == episodeKey(parsed.EpisodeNumbers), nil
}

func ensureEpisodeIdentityTx(ctx context.Context, tx *sql.Tx, parsed ParsedEpisode) (int64, error) {
	seriesID, err := ensureSeriesTx(ctx, tx, parsed.SeriesTitle, parsed.SeriesYear)
	if err != nil {
		return 0, err
	}
	seasonID, err := ensureSeasonTx(ctx, tx, seriesID, parsed.SeasonNumber)
	if err != nil {
		return 0, err
	}

	episodeIDs := make([]int64, 0, len(parsed.EpisodeNumbers))
	for _, number := range parsed.EpisodeNumbers {
		localTitle := ""
		if len(parsed.EpisodeNumbers) == 1 {
			localTitle = parsed.EpisodeTitle
		}
		id, err := ensureEpisodeTx(ctx, tx, seasonID, number, localTitle)
		if err != nil {
			return 0, err
		}
		episodeIDs = append(episodeIDs, id)
	}

	key := episodeKey(parsed.EpisodeNumbers)
	var itemID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM episode_items WHERE season_id = ? AND episode_key = ?`,
		seasonID, key).Scan(&itemID)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().Unix()
		result, err := tx.ExecContext(ctx, `
			INSERT INTO episode_items (
				season_id, episode_key, first_episode, last_episode, added_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?)`, seasonID, key,
			parsed.EpisodeNumbers[0], parsed.EpisodeNumbers[len(parsed.EpisodeNumbers)-1], now, now)
		if err != nil {
			return 0, err
		}
		itemID, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	}

	for ordinal, episodeID := range episodeIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO episode_item_members (episode_item_id, episode_id, ordinal)
			VALUES (?, ?, ?)
			ON CONFLICT(episode_item_id, episode_id) DO UPDATE SET ordinal = excluded.ordinal`,
			itemID, episodeID, ordinal); err != nil {
			return 0, err
		}
	}
	return itemID, nil
}

func ensureSeriesTx(ctx context.Context, tx *sql.Tx, title string, year int) (int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, title, year, tmdb_id FROM series ORDER BY id`)
	if err != nil {
		return 0, err
	}
	type seriesMatch struct {
		id     int64
		tmdbID sql.NullInt64
	}
	var matches []seriesMatch
	for rows.Next() {
		var id int64
		var candidateTitle string
		var candidateYear, tmdbID sql.NullInt64
		if err := rows.Scan(&id, &candidateTitle, &candidateYear, &tmdbID); err != nil {
			rows.Close()
			return 0, err
		}
		if normalizedTitle(candidateTitle) != normalizedTitle(title) {
			continue
		}
		if year > 0 {
			if candidateYear.Valid && int(candidateYear.Int64) == year {
				matches = append(matches, seriesMatch{id: id, tmdbID: tmdbID})
			}
		} else if !candidateYear.Valid {
			matches = append(matches, seriesMatch{id: id, tmdbID: tmdbID})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(matches) > 0 {
		identities := map[int64]bool{}
		var unproven []seriesMatch
		for _, match := range matches {
			if match.tmdbID.Valid {
				identities[match.tmdbID.Int64] = true
			} else {
				unproven = append(unproven, match)
			}
		}
		if len(identities) <= 1 {
			return matches[0].id, nil
		}
		if len(unproven) > 0 {
			return unproven[0].id, nil
		}
		// Two existing rows with contradictory proven identities make local
		// association unsafe. Keep a third row until TMDB identifies it rather
		// than silently choosing the oldest wrong answer.
	}

	now := time.Now().Unix()
	var storedYear any
	if year > 0 {
		storedYear = year
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO series (title, year, added_at, updated_at) VALUES (?, ?, ?, ?)`,
		title, storedYear, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func ensureSeasonTx(ctx context.Context, tx *sql.Tx, seriesID int64, number int) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM seasons WHERE series_id = ? AND season_number = ?`,
		seriesID, number).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	now := time.Now().Unix()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO seasons (series_id, season_number, added_at, updated_at)
		VALUES (?, ?, ?, ?)`, seriesID, number, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func ensureEpisodeTx(ctx context.Context, tx *sql.Tx, seasonID int64, number int, localTitle string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM episodes WHERE season_id = ? AND episode_number = ?`,
		seasonID, number).Scan(&id)
	if err == nil {
		if localTitle != "" {
			_, err = tx.ExecContext(ctx, `
				UPDATE episodes SET local_title = COALESCE(NULLIF(local_title, ''), ?)
				WHERE id = ?`, localTitle, id)
		}
		return id, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO episodes (season_id, episode_number, local_title)
		VALUES (?, ?, ?)`, seasonID, number, nullString(localTitle))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func episodeFileByPathTx(ctx context.Context, tx *sql.Tx, path string) (storedEpisodeFileIdentity, error) {
	var file storedEpisodeFileIdentity
	var primary int
	err := tx.QueryRowContext(ctx, `
		SELECT id, episode_item_id, size_bytes, modified_at, is_primary
		FROM episode_files WHERE path = ?`, path,
	).Scan(&file.ID, &file.ItemID, &file.SizeBytes, &file.Modified, &primary)
	file.Primary = primary != 0
	return file, err
}

func refreshKnownEpisodeFileTx(ctx context.Context, tx *sql.Tx, existing storedEpisodeFileIdentity,
	itemID int64, file scanner.File, generation int64,
) error {
	changed := existing.SizeBytes != file.SizeBytes || existing.Modified != file.ModifiedAt.Unix()
	primary := existing.Primary
	if existing.ItemID != itemID {
		needsPrimary, err := itemNeedsPrimaryTx(ctx, tx, itemID)
		if err != nil {
			return err
		}
		primary = needsPrimary
		if err := mergeEpisodeProgressTx(ctx, tx, existing.ItemID, itemID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE episode_files SET
			episode_item_id = ?, file_name = ?, size_bytes = ?, modified_at = ?,
			last_seen_scan = ?, is_primary = ?
		WHERE id = ?`, itemID, file.Name, file.SizeBytes, file.ModifiedAt.Unix(),
		generation, boolInt(primary), existing.ID); err != nil {
		return err
	}
	if changed {
		if err := invalidateEpisodeFileMediaTx(ctx, tx, existing.ID); err != nil {
			return err
		}
	}
	if existing.ItemID != itemID {
		if err := pruneEpisodeOrphansTx(ctx, tx); err != nil {
			return err
		}
		if err := repairEpisodePrimariesTx(ctx, tx); err != nil {
			return err
		}
	}
	return touchEpisodeHierarchyTx(ctx, tx, itemID)
}

func episodeMoveCandidateTx(ctx context.Context, tx *sql.Tx, targetItemID int64,
	file scanner.File, generation int64,
) (storedEpisodeFileIdentity, bool, error) {
	var targetSeasonID, targetSeriesID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT i.season_id, s.series_id
		FROM episode_items i JOIN seasons s ON s.id = i.season_id
		WHERE i.id = ?`, targetItemID).Scan(&targetSeasonID, &targetSeriesID); err != nil {
		return storedEpisodeFileIdentity{}, false, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT f.id, f.episode_item_id, i.season_id, s.series_id, f.path,
		       f.size_bytes, f.modified_at, f.is_primary
		FROM episode_files f
		JOIN episode_items i ON i.id = f.episode_item_id
		JOIN seasons s ON s.id = i.season_id
		WHERE f.size_bytes = ? AND f.modified_at = ? AND f.last_seen_scan < ?
		ORDER BY f.id`, file.SizeBytes, file.ModifiedAt.Unix(), generation)
	if err != nil {
		return storedEpisodeFileIdentity{}, false, err
	}
	defer rows.Close()
	var candidates []storedEpisodeFileIdentity
	for rows.Next() {
		var candidate storedEpisodeFileIdentity
		var primary int
		if err := rows.Scan(&candidate.ID, &candidate.ItemID, &candidate.SeasonID,
			&candidate.SeriesID, &candidate.Path, &candidate.SizeBytes,
			&candidate.Modified, &primary); err != nil {
			return storedEpisodeFileIdentity{}, false, err
		}
		candidate.Primary = primary != 0
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil || len(candidates) == 0 {
		return storedEpisodeFileIdentity{}, false, err
	}
	chosen := candidates[0]
	if len(candidates) > 1 {
		bestScore := -1
		bestCount := 0
		for _, candidate := range candidates {
			score := 0
			switch {
			case candidate.ItemID == targetItemID:
				score += 1000
			case candidate.SeasonID == targetSeasonID:
				score += 100
			case candidate.SeriesID == targetSeriesID:
				score += 10
			}
			if filepath.Clean(filepath.Dir(candidate.Path)) == filepath.Clean(filepath.Dir(file.Path)) {
				score += 1
			}
			switch {
			case score > bestScore:
				chosen, bestScore, bestCount = candidate, score, 1
			case score == bestScore:
				bestCount++
			}
		}
		if bestCount != 1 || bestScore == 0 {
			return storedEpisodeFileIdentity{}, false, nil
		}
	}
	if chosen.ItemID != targetItemID {
		var count int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM episode_files WHERE episode_item_id = ?`,
			chosen.ItemID).Scan(&count); err != nil {
			return storedEpisodeFileIdentity{}, false, err
		}
		if count > 1 {
			return storedEpisodeFileIdentity{}, false, nil
		}
	}
	return chosen, true, nil
}

func refreshMovedEpisodeFileTx(ctx context.Context, tx *sql.Tx, moved storedEpisodeFileIdentity,
	targetItemID int64, file scanner.File, generation int64,
) error {
	primary := moved.Primary
	if moved.ItemID != targetItemID {
		needsPrimary, err := itemNeedsPrimaryTx(ctx, tx, targetItemID)
		if err != nil {
			return err
		}
		primary = needsPrimary
		if err := mergeEpisodeProgressTx(ctx, tx, moved.ItemID, targetItemID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE episode_files SET
			episode_item_id = ?, path = ?, file_name = ?, size_bytes = ?, modified_at = ?,
			last_seen_scan = ?, is_primary = ?
		WHERE id = ?`, targetItemID, file.Path, file.Name, file.SizeBytes,
		file.ModifiedAt.Unix(), generation, boolInt(primary), moved.ID); err != nil {
		return err
	}
	if moved.ItemID != targetItemID {
		if err := pruneEpisodeOrphansTx(ctx, tx); err != nil {
			return err
		}
		if err := repairEpisodePrimariesTx(ctx, tx); err != nil {
			return err
		}
	}
	return touchEpisodeHierarchyTx(ctx, tx, targetItemID)
}

func itemNeedsPrimaryTx(ctx context.Context, tx *sql.Tx, itemID int64) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episode_files WHERE episode_item_id = ? AND is_primary = 1`,
		itemID).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func invalidateEpisodeFileMediaTx(ctx context.Context, tx *sql.Tx, fileID int64) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE episode_files SET
			media_status = 'pending', media_container = NULL,
			media_duration_seconds = NULL,
			video_stream_index = NULL, video_codec = NULL,
			video_width = NULL, video_height = NULL, video_frame_rate = NULL,
			media_inspected_at = 0
		WHERE id = ?`, fileID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		`DELETE FROM episode_file_audio_tracks WHERE episode_file_id = ?`, fileID)
	return err
}

func mergeEpisodeProgressTx(ctx context.Context, tx *sql.Tx, sourceID, targetID int64) error {
	if sourceID == targetID {
		return nil
	}
	var exists int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM episode_items WHERE id = ?`, sourceID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	// The duration describes the file and still lives on the item.
	if _, err := tx.ExecContext(ctx, `
		UPDATE episode_items SET duration_seconds = COALESCE(
			duration_seconds,
			(SELECT duration_seconds FROM episode_items WHERE id = ?)
		) WHERE id = ?`, sourceID, targetID); err != nil {
		return err
	}

	// The position does not. Each viewer's row moves on its own merit: the
	// comparison is per profile, so a renamed episode cannot hand one person's
	// position to another just because theirs happened to be more recent.
	_, err := tx.ExecContext(ctx, `
		INSERT INTO episode_progress (profile_id, episode_item_id, position_seconds, watched_at, finished)
		SELECT profile_id, ?, position_seconds, watched_at, finished
		FROM episode_progress WHERE episode_item_id = ?
		ON CONFLICT(profile_id, episode_item_id) DO UPDATE SET
			position_seconds = excluded.position_seconds,
			watched_at       = excluded.watched_at,
			finished         = excluded.finished
		WHERE excluded.watched_at > episode_progress.watched_at`, targetID, sourceID)
	return err
}

func touchEpisodeHierarchyTx(ctx context.Context, tx *sql.Tx, itemID int64) error {
	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE episode_items SET updated_at = ? WHERE id = ?`, now, itemID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE seasons SET updated_at = ?
		WHERE id = (SELECT season_id FROM episode_items WHERE id = ?);
		`, now, itemID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE series SET updated_at = ?
		WHERE id = (
			SELECT s.series_id FROM seasons s
			JOIN episode_items i ON i.season_id = s.id WHERE i.id = ?
		)`, now, itemID)
	return err
}

func removeMoviePathTx(ctx context.Context, tx *sql.Tx, path string) error {
	var movieID int64
	var primary int
	err := tx.QueryRowContext(ctx, `
		SELECT movie_id, is_primary FROM movie_files WHERE path = ?`, path).Scan(&movieID, &primary)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM movie_files WHERE path = ?`, path); err != nil {
		return err
	}
	var remaining int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM movie_files WHERE movie_id = ?`, movieID).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM movies WHERE id = ?`, movieID)
		return err
	}
	if primary != 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE movie_files SET is_primary = 1
			WHERE id = (SELECT MIN(id) FROM movie_files WHERE movie_id = ?)`, movieID); err != nil {
			return err
		}
		return mirrorPrimaryTx(ctx, tx, movieID)
	}
	return nil
}

func removeEpisodePathTx(ctx context.Context, tx *sql.Tx, path string) error {
	var itemID int64
	var primary int
	err := tx.QueryRowContext(ctx, `
		SELECT episode_item_id, is_primary FROM episode_files WHERE path = ?`, path).Scan(&itemID, &primary)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM episode_files WHERE path = ?`, path); err != nil {
		return err
	}
	if primary != 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE episode_files SET is_primary = 1
			WHERE id = (SELECT MIN(id) FROM episode_files WHERE episode_item_id = ?)`, itemID); err != nil {
			return err
		}
	}
	return pruneEpisodeOrphansTx(ctx, tx)
}

func (s *Store) DeleteEpisodeFilesNotSeenIn(ctx context.Context, generation int64) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("removing episode files that disappeared: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var removed int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM episode_files WHERE last_seen_scan < ?`, generation).Scan(&removed); err != nil {
		return 0, fmt.Errorf("removing episode files that disappeared: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM episode_files WHERE last_seen_scan < ?`, generation); err != nil {
		return 0, fmt.Errorf("removing episode files that disappeared: %w", err)
	}
	if err := pruneEpisodeOrphansTx(ctx, tx); err != nil {
		return 0, fmt.Errorf("removing orphaned episode rows: %w", err)
	}
	if err := repairEpisodePrimariesTx(ctx, tx); err != nil {
		return 0, fmt.Errorf("repairing episode primary files: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("removing episode files that disappeared: %w", err)
	}
	return removed, nil
}

func pruneEpisodeOrphansTx(ctx context.Context, tx *sql.Tx) error {
	statements := []string{
		`DELETE FROM episode_items
		 WHERE NOT EXISTS (SELECT 1 FROM episode_files f WHERE f.episode_item_id = episode_items.id)`,
		`DELETE FROM episodes
		 WHERE NOT EXISTS (SELECT 1 FROM episode_item_members m WHERE m.episode_id = episodes.id)`,
		`DELETE FROM seasons
		 WHERE NOT EXISTS (SELECT 1 FROM episode_items i WHERE i.season_id = seasons.id)`,
		`DELETE FROM series
		 WHERE NOT EXISTS (SELECT 1 FROM seasons s WHERE s.series_id = series.id)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func repairEpisodePrimariesTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE episode_files SET is_primary = 1
		WHERE id IN (
			SELECT MIN(f.id)
			FROM episode_files f
			GROUP BY f.episode_item_id
			HAVING SUM(CASE WHEN f.is_primary = 1 THEN 1 ELSE 0 END) = 0
		)`)
	return err
}
