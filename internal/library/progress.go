package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Rules for what counts as watched, and for what is worth remembering.
const (
	// Never ask for more than the last two minutes. Credit sequences run longer
	// than that, so stopping when they start counts as having watched the film,
	// and nobody should have to sit through five per cent of a three-hour epic
	// to clear it from the row.
	finishedRemainingSeconds = 120.0

	// …but never accept less than the last five per cent either, which is what
	// keeps a ten-minute short from being called finished at eight minutes.
	finishedFraction = 0.95

	// Below this, nothing is remembered. Opening a film to look at the poster
	// and closing it again should not fill the continue-watching row.
	minimumRememberedSeconds = 30.0
)

// Progress is where a viewer got to in one film.
//
// WatchedAt is a pointer so that "never watched" is absent from the JSON rather
// than the year 1: omitempty does nothing for a struct, and a client checking
// truthiness would read 0001-01-01 as a real date.
type Progress struct {
	PositionSeconds float64    `json:"position_seconds"`
	DurationSeconds float64    `json:"duration_seconds,omitempty"`
	Finished        bool       `json:"finished"`
	WatchedAt       *time.Time `json:"watched_at,omitempty"`
}

// finishedRule decides whether a position counts as having watched the film.
//
// Recomputed on every report rather than latched, so that starting a film again
// clears it and the film returns to the continue-watching row -- which is what
// anybody rewatching something expects.
// The two thresholds combine by taking whichever sits *closer to the end*, not
// whichever triggers first. Applied the other way round, the fixed two-minute
// window called a ten-minute short finished at eight minutes.
func finishedRule(position, duration float64) bool {
	if duration <= 0 {
		// Without a duration there is nothing to be near the end of. The film
		// stays in the row until the viewer stops opening it.
		return false
	}
	remaining := min(finishedRemainingSeconds, duration*(1-finishedFraction))
	return duration-position <= remaining
}

// SaveProgress records a playback position.
//
// The duration is taken from the row when the caller does not know one -- the
// player knows it for a direct stream and usually not for a remuxed one.
func (s *Store) SaveProgress(ctx context.Context, profileID, id int64, position, duration float64, now time.Time) (Progress, error) {
	if position < 0 {
		position = 0
	}

	var stored sql.NullFloat64
	var isDefault int
	if err := s.db.QueryRowContext(ctx, `
		SELECT m.duration_seconds, p.is_default
		FROM movies AS m
		JOIN profiles AS p ON p.id = ?
		WHERE m.id = ?`, profileID, id).Scan(&stored, &isDefault); err != nil {
		if err == sql.ErrNoRows {
			return Progress{}, ErrNoSuchMovie
		}
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}

	if duration <= 0 && stored.Valid {
		duration = stored.Float64
	}

	finished := finishedRule(position, duration)

	// Below the floor, the position is not worth keeping -- but a duration
	// learned along the way is, so it is still written.
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO playback_progress
			(profile_id, movie_id, position_seconds, watched_at, finished)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, movie_id) DO UPDATE SET
			position_seconds = excluded.position_seconds,
			watched_at       = excluded.watched_at,
			finished         = excluded.finished`,
		profileID, id, remembered, watchedAt, boolToInt(finished),
	); err != nil {
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}

	if isDefault != 0 {
		// v1.2 reads these legacy columns. Keeping them in step makes the
		// updater's executable rollback real rather than ceremonial.
		_, err = tx.ExecContext(ctx, `
			UPDATE movies SET
				position_seconds = ?,
				duration_seconds = COALESCE(?, duration_seconds),
				watched_at       = ?,
				finished         = ?
			WHERE id = ?`,
			remembered, durationValue, watchedAt, boolToInt(finished), id)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE movies SET duration_seconds = COALESCE(?, duration_seconds) WHERE id = ?`,
			durationValue, id)
	}
	if err != nil {
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}

	p := Progress{PositionSeconds: remembered, DurationSeconds: duration, Finished: finished}
	if watchedAt > 0 {
		at := time.Unix(watchedAt, 0).UTC()
		p.WatchedAt = &at
	}
	return p, nil
}

// ResetProgress forgets where a viewer got to, which is what "start from the
// beginning" does. The duration is kept: it describes the file, not the viewing.
func (s *Store) ResetProgress(ctx context.Context, profileID, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var isDefault int
	if err := tx.QueryRowContext(ctx, `
		SELECT p.is_default
		FROM movies AS m
		JOIN profiles AS p ON p.id = ?
		WHERE m.id = ?`, profileID, id).Scan(&isDefault); errors.Is(err, sql.ErrNoRows) {
		return ErrNoSuchMovie
	} else if err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM playback_progress WHERE profile_id = ? AND movie_id = ?`, profileID, id)
	if err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}
	if isDefault != 0 {
		if _, err := tx.ExecContext(ctx,
			`UPDATE movies SET position_seconds = 0, watched_at = 0, finished = 0 WHERE id = ?`,
			id); err != nil {
			return fmt.Errorf("resetting progress for film %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}
	return nil
}

// SaveDuration records a duration learned from probing a file, without
// touching the viewing position.
func (s *Store) SaveDuration(ctx context.Context, id int64, seconds float64) error {
	if seconds <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE movies SET duration_seconds = ? WHERE id = ?`, seconds, id)
	if err != nil {
		return fmt.Errorf("saving the duration of film %d: %w", id, err)
	}
	return nil
}

// ContinueWatching returns films that were started and not finished, most
// recently watched first.
func (s *Store) ContinueWatching(ctx context.Context, profileID int64, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		`+profileProgressJoin+`
		WHERE COALESCE(p.finished, 0) = 0
		  AND COALESCE(p.watched_at, 0) > 0
		  AND COALESCE(p.position_seconds, 0) >= ?
		ORDER BY p.watched_at DESC, m.id DESC
		LIMIT ?`, profileID, minimumRememberedSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("listing films in progress: %w", err)
	}
	return collectMovies(rows, limit)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
