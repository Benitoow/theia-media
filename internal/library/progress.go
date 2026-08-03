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
	if err := s.db.QueryRowContext(ctx,
		`SELECT duration_seconds FROM movies WHERE id = ?`, id).Scan(&stored); err != nil {
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
	defer tx.Rollback()

	// The duration describes the file, so it stays on the film even when the
	// position that taught us about it belongs to one viewer.
	if _, err := tx.ExecContext(ctx,
		`UPDATE movies SET duration_seconds = COALESCE(?, duration_seconds) WHERE id = ?`,
		durationValue, id,
	); err != nil {
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO movie_progress (profile_id, movie_id, position_seconds, watched_at, finished)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(profile_id, movie_id) DO UPDATE SET
			position_seconds = excluded.position_seconds,
			watched_at       = excluded.watched_at,
			finished         = excluded.finished`,
		profileID, id, remembered, watchedAt, boolToInt(finished),
	); err != nil {
		return Progress{}, fmt.Errorf("saving progress for film %d: %w", id, err)
	}

	if err := mirrorLegacyMovieProgress(ctx, tx, profileID, id,
		remembered, watchedAt, boolToInt(finished)); err != nil {
		return Progress{}, err
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
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM movies WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoSuchMovie
	}
	if err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}
	defer tx.Rollback()

	// Deleting rather than zeroing: a viewer who never watched a film and one
	// who reset it are the same state, and keeping a row for each would grow a
	// table nothing reads.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM movie_progress WHERE profile_id = ? AND movie_id = ?`, profileID, id,
	); err != nil {
		return fmt.Errorf("resetting progress for film %d: %w", id, err)
	}
	if err := mirrorLegacyMovieProgress(ctx, tx, profileID, id, 0, 0, 0); err != nil {
		return err
	}
	return tx.Commit()
}

// mirrorLegacyMovieProgress keeps movies.position_seconds, watched_at and
// finished in step with the *default* profile.
//
// Those columns are dead weight for this binary, which reads movie_progress.
// They are not dead weight for the previous one: the updater keeps it as a
// rollback target and v1.5.0 selects them at startup, so leaving them frozen
// would silently roll a viewer's history back to whatever it was on the day
// they updated. Decision 31 learned this the same way and the reasoning has not
// expired.
func mirrorLegacyMovieProgress(ctx context.Context, tx *sql.Tx, profileID, movieID int64,
	position float64, watchedAt int64, finished int,
) error {
	var defaultID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM profiles ORDER BY id LIMIT 1`).Scan(&defaultID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("mirroring legacy progress for film %d: %w", movieID, err)
	}
	if defaultID != profileID {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET position_seconds = ?, watched_at = ?, finished = ?
		WHERE id = ?`, position, watchedAt, finished, movieID); err != nil {
		return fmt.Errorf("mirroring legacy progress for film %d: %w", movieID, err)
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
		SELECT `+movieColumns+movieSource+`
		WHERE p.finished = 0
		  AND p.watched_at > 0
		  AND p.position_seconds >= ?
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
