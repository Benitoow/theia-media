package profiles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Store owns the profiles table and the two progress tables that hang off it.
type Store struct {
	db *sql.DB
}

func New(database *sql.DB) *Store {
	return &Store{db: database}
}

// List returns every profile, oldest first, so the chooser keeps a stable order
// between visits. Avatar bytes are never loaded here: eight pictures is a
// megabyte of JSON nobody asked for.
func (s *Store) List(ctx context.Context) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, avatar_bytes IS NOT NULL, avatar_version, created_at
		FROM profiles
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	defer rows.Close()

	out := make([]Profile, 0, MaxProfiles)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("listing profiles: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	return out, nil
}

// Get returns one profile with the local facts its page shows.
func (s *Store) Get(ctx context.Context, id int64) (Profile, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, avatar_bytes IS NOT NULL, avatar_version, created_at
		FROM profiles WHERE id = ?`, id)

	p, err := scanProfile(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Profile{}, ErrNoSuchProfile
	case err != nil:
		return Profile{}, fmt.Errorf("reading profile %d: %w", id, err)
	}

	stats, err := s.stats(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	p.Stats = &stats
	return p, nil
}

// Exists is the cheap check the progress routes need on every save. It does not
// authenticate anything; it only refuses to write a history against an id that
// is not there, which would otherwise accumulate invisibly.
func (s *Store) Exists(ctx context.Context, id int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM profiles WHERE id = ?`, id).Scan(&one)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("checking profile %d: %w", id, err)
	}
	return true, nil
}

// DefaultID is the profile a request without an explicit one falls back to: the
// oldest row. It exists so that the released frontend, the home rows and any
// client that has never heard of profiles keep working unchanged.
func (s *Store) DefaultID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM profiles ORDER BY id LIMIT 1`).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, ErrNoSuchProfile
	case err != nil:
		return 0, fmt.Errorf("reading the default profile: %w", err)
	}
	return id, nil
}

func (s *Store) Create(ctx context.Context, name string) (Profile, error) {
	clean, err := CleanName(name)
	if err != nil {
		return Profile{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&count); err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	if count >= MaxProfiles {
		return Profile{}, ErrProfileLimit
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO profiles (name) VALUES (?)`, clean)
	if err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	return s.Get(ctx, id)
}

// Rename also takes the default profile out of its unnamed state, which is the
// only way that state is ever left. There is no way back into it, by design: a
// profile the viewer has named should not silently become "the default" again.
func (s *Store) Rename(ctx context.Context, id int64, name string) (Profile, error) {
	clean, err := CleanName(name)
	if err != nil {
		return Profile{}, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE profiles SET name = ? WHERE id = ?`, clean, id)
	if err != nil {
		return Profile{}, fmt.Errorf("renaming profile %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Profile{}, ErrNoSuchProfile
	}
	return s.Get(ctx, id)
}

// Delete removes a profile and, through the foreign keys, its whole history.
// The last profile is refused: a library with nobody to watch it has no state
// to write progress against, and the chooser would have nothing to offer.
func (s *Store) Delete(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	defer tx.Rollback()

	// Existence is checked before the last-profile rule, and the order matters:
	// the other way round, deleting an id that does not exist reported "the last
	// profile cannot be deleted" whenever one profile remained -- a true
	// sentence about the wrong subject, which is worse than a plain 404.
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM profiles WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoSuchProfile
	}
	if err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&count); err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	if count <= 1 {
		return ErrLastProfile
	}

	var previousDefault int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM profiles ORDER BY id LIMIT 1`).Scan(&previousDefault); err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchProfile
	}

	// Deleting the default profile promotes the next oldest, and the legacy
	// columns on `movies` follow whoever is default -- they are what a v1.5.0
	// binary reads after a rollback. Left alone they would keep serving the
	// deleted viewer's positions, which is a history belonging to nobody.
	if id == previousDefault {
		if _, err := tx.ExecContext(ctx, `
			UPDATE movies SET
				position_seconds = COALESCE((SELECT position_seconds FROM movie_progress
					WHERE movie_id = movies.id
					  AND profile_id = (SELECT id FROM profiles ORDER BY id LIMIT 1)), 0),
				watched_at = COALESCE((SELECT watched_at FROM movie_progress
					WHERE movie_id = movies.id
					  AND profile_id = (SELECT id FROM profiles ORDER BY id LIMIT 1)), 0),
				finished = COALESCE((SELECT finished FROM movie_progress
					WHERE movie_id = movies.id
					  AND profile_id = (SELECT id FROM profiles ORDER BY id LIMIT 1)), 0)`,
		); err != nil {
			return fmt.Errorf("re-mirroring legacy progress after deleting profile %d: %w", id, err)
		}
	}
	return tx.Commit()
}

// SetAvatar stores an already-normalised picture and bumps its version, which
// is what lets the URL be cached immutably and still change.
func (s *Store) SetAvatar(ctx context.Context, id int64, image []byte, contentType string) (Profile, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE profiles
		SET avatar_bytes = ?, avatar_type = ?, avatar_version = avatar_version + 1
		WHERE id = ?`, image, contentType, id)
	if err != nil {
		return Profile{}, fmt.Errorf("saving the picture of profile %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Profile{}, ErrNoSuchProfile
	}
	return s.Get(ctx, id)
}

func (s *Store) ClearAvatar(ctx context.Context, id int64) (Profile, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE profiles
		SET avatar_bytes = NULL, avatar_type = NULL, avatar_version = avatar_version + 1
		WHERE id = ?`, id)
	if err != nil {
		return Profile{}, fmt.Errorf("removing the picture of profile %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Profile{}, ErrNoSuchProfile
	}
	return s.Get(ctx, id)
}

func (s *Store) Avatar(ctx context.Context, id int64) (Avatar, error) {
	var (
		blob    []byte
		kind    sql.NullString
		version int64
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT avatar_bytes, avatar_type, avatar_version FROM profiles WHERE id = ?`, id,
	).Scan(&blob, &kind, &version)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Avatar{}, ErrNoSuchProfile
	case err != nil:
		return Avatar{}, fmt.Errorf("reading the picture of profile %d: %w", id, err)
	}
	if len(blob) == 0 {
		return Avatar{}, ErrNoAvatar
	}
	contentType := kind.String
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return Avatar{Bytes: blob, ContentType: contentType, Version: version}, nil
}

func (s *Store) stats(ctx context.Context, id int64) (Stats, error) {
	var (
		out       Stats
		watchedAt sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM movie_progress
			   WHERE profile_id = ? AND finished = 0 AND watched_at > 0),
			(SELECT COUNT(*) FROM movie_progress
			   WHERE profile_id = ? AND finished = 1),
			(SELECT COUNT(*) FROM episode_progress
			   WHERE profile_id = ? AND finished = 0 AND watched_at > 0),
			(SELECT COUNT(*) FROM episode_progress
			   WHERE profile_id = ? AND finished = 1),
			(SELECT MAX(watched_at) FROM (
			   SELECT watched_at FROM movie_progress WHERE profile_id = ?
			   UNION ALL
			   SELECT watched_at FROM episode_progress WHERE profile_id = ?))`,
		id, id, id, id, id, id,
	).Scan(&out.MoviesStarted, &out.MoviesFinished,
		&out.EpisodesStarted, &out.EpisodesFinished, &watchedAt)
	if err != nil {
		return Stats{}, fmt.Errorf("reading the history of profile %d: %w", id, err)
	}
	if watchedAt.Valid && watchedAt.Int64 > 0 {
		at := time.Unix(watchedAt.Int64, 0).UTC()
		out.LastWatchedAt = &at
	}
	return out, nil
}

// CleanName trims and validates. A name made only of spaces is not a name, and
// control characters in a filename-like field have caused enough trouble
// elsewhere in this project to be refused here on sight.
func CleanName(name string) (string, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "", ErrInvalidName
	}
	if utf8.RuneCountInString(clean) > MaxNameRunes {
		return "", ErrInvalidName
	}
	for _, r := range clean {
		if r < 0x20 || r == 0x7f {
			return "", ErrInvalidName
		}
	}
	return clean, nil
}

func scanProfile(row interface{ Scan(...any) error }) (Profile, error) {
	var (
		p         Profile
		name      sql.NullString
		hasAvatar bool
		createdAt int64
	)
	if err := row.Scan(&p.ID, &name, &hasAvatar, &p.AvatarVersion, &createdAt); err != nil {
		return Profile{}, err
	}
	p.Name = name.String
	p.IsDefault = !name.Valid
	p.HasAvatar = hasAvatar
	p.CreatedAt = time.Unix(createdAt, 0).UTC()
	return p, nil
}
