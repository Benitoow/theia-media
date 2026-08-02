package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ConsolidateSeries merges only identities proven by the same non-null TMDB
// series id. Local title similarity is useful during association, but it is not
// proof strong enough to fold remakes together after the fact.
func (s *Store) ConsolidateSeries(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tmdb_id, id FROM series
		WHERE tmdb_id IS NOT NULL
		ORDER BY tmdb_id, id`)
	if err != nil {
		return 0, fmt.Errorf("finding duplicate series: %w", err)
	}
	groups := map[int64][]int64{}
	for rows.Next() {
		var tmdbID, id int64
		if err := rows.Scan(&tmdbID, &id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("finding duplicate series: %w", err)
		}
		groups[tmdbID] = append(groups[tmdbID], id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("finding duplicate series: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("finding duplicate series: %w", err)
	}

	merged := 0
	for _, ids := range groups {
		if len(ids) < 2 {
			continue
		}
		canonical := ids[0]
		for _, source := range ids[1:] {
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				return merged, fmt.Errorf("merging duplicate series: %w", err)
			}
			if err := mergeSeriesTx(ctx, tx, source, canonical); err != nil {
				tx.Rollback() //nolint:errcheck
				return merged, fmt.Errorf("merging series %d into %d: %w", source, canonical, err)
			}
			if err := tx.Commit(); err != nil {
				return merged, fmt.Errorf("merging series %d into %d: %w", source, canonical, err)
			}
			merged++
		}
	}
	return merged, nil
}

func mergeSeriesTx(ctx context.Context, tx *sql.Tx, sourceID, targetID int64) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE series SET
			year = COALESCE(year, (SELECT year FROM series WHERE id = ?)),
			tmdb_name = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT tmdb_name FROM series WHERE id = ?) ELSE tmdb_name END,
			original_name = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT original_name FROM series WHERE id = ?) ELSE original_name END,
			overview = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT overview FROM series WHERE id = ?) ELSE overview END,
			first_air_date = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT first_air_date FROM series WHERE id = ?) ELSE first_air_date END,
			poster_path = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT poster_path FROM series WHERE id = ?) ELSE poster_path END,
			backdrop_path = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT backdrop_path FROM series WHERE id = ?) ELSE backdrop_path END,
			vote_average = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT vote_average FROM series WHERE id = ?) ELSE vote_average END,
			genres_json = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT genres_json FROM series WHERE id = ?) ELSE genres_json END,
			cast_json = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT cast_json FROM series WHERE id = ?) ELSE cast_json END,
			creators_json = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT creators_json FROM series WHERE id = ?) ELSE creators_json END,
			metadata_status = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM series WHERE id = ?)
				THEN (SELECT metadata_status FROM series WHERE id = ?) ELSE metadata_status END,
			metadata_fetched_at = MAX(metadata_fetched_at,
				(SELECT metadata_fetched_at FROM series WHERE id = ?)),
			added_at = MIN(added_at, (SELECT added_at FROM series WHERE id = ?)),
			updated_at = MAX(updated_at, (SELECT updated_at FROM series WHERE id = ?))
		WHERE id = ?`, repeatedMergeArgs(sourceID, 26, targetID)...); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, season_number FROM seasons WHERE series_id = ? ORDER BY season_number`, sourceID)
	if err != nil {
		return err
	}
	type seasonRow struct {
		id     int64
		number int
	}
	var seasons []seasonRow
	for rows.Next() {
		var season seasonRow
		if err := rows.Scan(&season.id, &season.number); err != nil {
			rows.Close()
			return err
		}
		seasons = append(seasons, season)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, sourceSeason := range seasons {
		var targetSeasonID int64
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM seasons WHERE series_id = ? AND season_number = ?`,
			targetID, sourceSeason.number).Scan(&targetSeasonID)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx,
				`UPDATE seasons SET series_id = ? WHERE id = ?`, targetID, sourceSeason.id); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := mergeSeasonTx(ctx, tx, sourceSeason.id, targetSeasonID); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM series WHERE id = ?`, sourceID); err != nil {
		return err
	}
	if err := pruneEpisodeOrphansTx(ctx, tx); err != nil {
		return err
	}
	return repairEpisodePrimariesTx(ctx, tx)
}

func mergeSeasonTx(ctx context.Context, tx *sql.Tx, sourceID, targetID int64) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE seasons SET
			tmdb_id = COALESCE(tmdb_id, (SELECT tmdb_id FROM seasons WHERE id = ?)),
			name = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM seasons WHERE id = ?)
				THEN (SELECT name FROM seasons WHERE id = ?) ELSE name END,
			overview = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM seasons WHERE id = ?)
				THEN (SELECT overview FROM seasons WHERE id = ?) ELSE overview END,
			air_date = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM seasons WHERE id = ?)
				THEN (SELECT air_date FROM seasons WHERE id = ?) ELSE air_date END,
			poster_path = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM seasons WHERE id = ?)
				THEN (SELECT poster_path FROM seasons WHERE id = ?) ELSE poster_path END,
			episode_count = MAX(COALESCE(episode_count, 0),
				COALESCE((SELECT episode_count FROM seasons WHERE id = ?), 0)),
			metadata_status = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM seasons WHERE id = ?)
				THEN (SELECT metadata_status FROM seasons WHERE id = ?) ELSE metadata_status END,
			metadata_fetched_at = MAX(metadata_fetched_at,
				(SELECT metadata_fetched_at FROM seasons WHERE id = ?)),
			added_at = MIN(added_at, (SELECT added_at FROM seasons WHERE id = ?)),
			updated_at = MAX(updated_at, (SELECT updated_at FROM seasons WHERE id = ?))
		WHERE id = ?`, repeatedMergeArgs(sourceID, 15, targetID)...); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, episode_number FROM episodes WHERE season_id = ? ORDER BY episode_number`, sourceID)
	if err != nil {
		return err
	}
	type episodeRow struct {
		id     int64
		number int
	}
	var episodes []episodeRow
	for rows.Next() {
		var episode episodeRow
		if err := rows.Scan(&episode.id, &episode.number); err != nil {
			rows.Close()
			return err
		}
		episodes = append(episodes, episode)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, sourceEpisode := range episodes {
		var targetEpisodeID int64
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM episodes WHERE season_id = ? AND episode_number = ?`,
			targetID, sourceEpisode.number).Scan(&targetEpisodeID)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx,
				`UPDATE episodes SET season_id = ? WHERE id = ?`, targetID, sourceEpisode.id); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := mergeEpisodeMetadataTx(ctx, tx, sourceEpisode.id, targetEpisodeID); err != nil {
			return err
		}
	}

	itemRows, err := tx.QueryContext(ctx, `
		SELECT id, episode_key FROM episode_items WHERE season_id = ? ORDER BY id`, sourceID)
	if err != nil {
		return err
	}
	type itemRow struct {
		id  int64
		key string
	}
	var items []itemRow
	for itemRows.Next() {
		var item itemRow
		if err := itemRows.Scan(&item.id, &item.key); err != nil {
			itemRows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return err
	}
	if err := itemRows.Close(); err != nil {
		return err
	}

	for _, sourceItem := range items {
		var targetItemID int64
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM episode_items WHERE season_id = ? AND episode_key = ?`,
			targetID, sourceItem.key).Scan(&targetItemID)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx,
				`UPDATE episode_items SET season_id = ? WHERE id = ?`, targetID, sourceItem.id); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := mergeEpisodeProgressTx(ctx, tx, sourceItem.id, targetItemID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE episode_files SET is_primary = 0 WHERE episode_item_id = ?`,
			sourceItem.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE episode_files SET episode_item_id = ? WHERE episode_item_id = ?`,
			targetItemID, sourceItem.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM episode_items WHERE id = ?`, sourceItem.id); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM seasons WHERE id = ?`, sourceID)
	return err
}

func mergeEpisodeMetadataTx(ctx context.Context, tx *sql.Tx, sourceID, targetID int64) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT episode_item_id, ordinal FROM episode_item_members WHERE episode_id = ?`, sourceID)
	if err != nil {
		return err
	}
	type memberRow struct {
		itemID  int64
		ordinal int
	}
	var members []memberRow
	for rows.Next() {
		var member memberRow
		if err := rows.Scan(&member.itemID, &member.ordinal); err != nil {
			rows.Close()
			return err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE episodes SET
			local_title = COALESCE(NULLIF(local_title, ''),
				(SELECT local_title FROM episodes WHERE id = ?)),
			tmdb_id = COALESCE(tmdb_id, (SELECT tmdb_id FROM episodes WHERE id = ?)),
			name = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?)
				THEN (SELECT name FROM episodes WHERE id = ?) ELSE name END,
			overview = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?)
				THEN (SELECT overview FROM episodes WHERE id = ?) ELSE overview END,
			air_date = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?)
				THEN (SELECT air_date FROM episodes WHERE id = ?) ELSE air_date END,
			still_path = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?)
				THEN (SELECT still_path FROM episodes WHERE id = ?) ELSE still_path END,
			runtime_minutes = COALESCE(runtime_minutes,
				(SELECT runtime_minutes FROM episodes WHERE id = ?)),
			vote_average = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?)
				THEN (SELECT vote_average FROM episodes WHERE id = ?) ELSE vote_average END,
			metadata_status = CASE WHEN metadata_status != 'ok' OR metadata_fetched_at <
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?)
				THEN (SELECT metadata_status FROM episodes WHERE id = ?) ELSE metadata_status END,
			metadata_fetched_at = MAX(metadata_fetched_at,
				(SELECT metadata_fetched_at FROM episodes WHERE id = ?))
		WHERE id = ?`, repeatedMergeArgs(sourceID, 16, targetID)...); err != nil {
		return err
	}
	for _, member := range members {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM episode_item_members WHERE episode_item_id = ? AND episode_id = ?`,
			member.itemID, sourceID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO episode_item_members (episode_item_id, episode_id, ordinal)
			VALUES (?, ?, ?)`, member.itemID, targetID, member.ordinal); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM episodes WHERE id = ?`, sourceID)
	return err
}

func repeatedMergeArgs(sourceID int64, sourceCount int, targetID int64) []any {
	args := make([]any, 0, sourceCount+1)
	for range sourceCount {
		args = append(args, sourceID)
	}
	return append(args, targetID)
}
