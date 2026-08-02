package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SaveEpisodeProgress records progress on the playable item, not on each TMDB
// member. A combined S01E01E02 file therefore has one honest resume position.
func (s *Store) SaveEpisodeProgress(ctx context.Context, id int64, position, duration float64, now time.Time) (Progress, error) {
	if position < 0 {
		position = 0
	}

	var stored sql.NullFloat64
	if err := s.db.QueryRowContext(ctx,
		`SELECT duration_seconds FROM episode_items WHERE id = ?`, id).Scan(&stored); err != nil {
		if err == sql.ErrNoRows {
			return Progress{}, ErrNoSuchEpisodeItem
		}
		return Progress{}, fmt.Errorf("saving progress for episode %d: %w", id, err)
	}
	if duration <= 0 && stored.Valid {
		duration = stored.Float64
	}

	finished := finishedRule(position, duration)
	remembered := position
	watchedAt := now.Unix()
	if position < minimumRememberedSeconds && !finished {
		remembered = 0
		watchedAt = 0
	}

	var durationValue any
	if duration > 0 {
		durationValue = duration
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE episode_items SET
			position_seconds = ?,
			duration_seconds = COALESCE(?, duration_seconds),
			watched_at = ?,
			finished = ?,
			updated_at = ?
		WHERE id = ?`,
		remembered, durationValue, watchedAt, boolToInt(finished), now.Unix(), id); err != nil {
		return Progress{}, fmt.Errorf("saving progress for episode %d: %w", id, err)
	}

	progress := Progress{
		PositionSeconds: remembered,
		DurationSeconds: duration,
		Finished:        finished,
	}
	if watchedAt > 0 {
		at := time.Unix(watchedAt, 0).UTC()
		progress.WatchedAt = &at
	}
	return progress, nil
}

func (s *Store) ResetEpisodeProgress(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE episode_items SET
			position_seconds = 0, watched_at = 0, finished = 0, updated_at = ?
		WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("resetting progress for episode %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchEpisodeItem
	}
	return nil
}

func (s *Store) SaveEpisodeDuration(ctx context.Context, id int64, seconds float64) error {
	if seconds <= 0 {
		return nil
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE episode_items SET duration_seconds = ?, updated_at = ? WHERE id = ?`,
		seconds, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("saving the duration of episode %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchEpisodeItem
	}
	return nil
}
