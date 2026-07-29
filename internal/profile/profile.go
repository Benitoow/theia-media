// Package profile stores cosmetic household profiles.
//
// Profiles deliberately have no credentials or permissions. Anyone who can
// reach Theia on the local network may list, create, edit, select or remove
// them. Their only functional effect is choosing a playback-progress namespace.
package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaximumCount keeps an unauthenticated LAN endpoint from growing the
	// database without bound while leaving more than enough room for a home.
	MaximumCount = 12

	// MaximumNameRunes is a display constraint as well as a storage one. Names
	// have to survive a television navigation pill without becoming a paragraph.
	MaximumNameRunes = 40
)

var (
	ErrNoSuchProfile   = errors.New("no such profile")
	ErrInvalidName     = errors.New("invalid profile name")
	ErrTooManyProfiles = errors.New("too many profiles")
	ErrDefaultProfile  = errors.New("the default profile cannot be deleted")
	ErrLastProfile     = errors.New("the last profile cannot be deleted")
	ErrNoAvatar        = errors.New("profile has no avatar")
)

// Profile is one freely selectable viewer identity.
//
// Name is nil only for the built-in default profile until somebody renames it.
// That lets each frontend locale supply its own default label.
type Profile struct {
	ID            int64   `json:"id"`
	Name          *string `json:"name"`
	Default       bool    `json:"is_default"`
	HasAvatar     bool    `json:"has_avatar"`
	AvatarVersion int64   `json:"avatar_version"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
}

// Avatar is the locally uploaded image for a profile.
type Avatar struct {
	Data        []byte
	MediaType   string
	Version     int64
	LastUpdated time.Time
}

// Store persists profiles in the main Theia database.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// NormalizeName trims a profile name and enforces the API's one shared rule.
func NormalizeName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > MaximumNameRunes {
		return "", ErrInvalidName
	}
	return name, nil
}

// List returns the compatibility default first, then the remaining profiles in
// creation order.
func (s *Store) List(ctx context.Context) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, is_default, avatar_data IS NOT NULL, avatar_version,
		       created_at, updated_at
		FROM profiles
		ORDER BY is_default DESC, created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]Profile, 0, 4)
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("listing profiles: %w", err)
		}
		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	return profiles, nil
}

// Get returns one profile.
func (s *Store) Get(ctx context.Context, id int64) (Profile, error) {
	p, err := scanProfile(s.db.QueryRowContext(ctx, `
		SELECT id, name, is_default, avatar_data IS NOT NULL, avatar_version,
		       created_at, updated_at
		FROM profiles
		WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNoSuchProfile
	}
	if err != nil {
		return Profile{}, fmt.Errorf("reading profile %d: %w", id, err)
	}
	return p, nil
}

// DefaultID returns the compatibility profile used when a client sends no
// selection at all.
func (s *Store) DefaultID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM profiles WHERE is_default = 1 LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoSuchProfile
	}
	if err != nil {
		return 0, fmt.Errorf("reading the default profile: %w", err)
	}
	return id, nil
}

// Create adds a named profile.
func (s *Store) Create(ctx context.Context, value string) (Profile, error) {
	name, err := NormalizeName(value)
	if err != nil {
		return Profile{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM profiles`).Scan(&count); err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	if count >= MaximumCount {
		return Profile{}, ErrTooManyProfiles
	}

	now := time.Now().Unix()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO profiles (name, is_default, created_at, updated_at)
		VALUES (?, 0, ?, ?)`, name, now, now)
	if err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Profile{}, fmt.Errorf("creating a profile: %w", err)
	}
	return s.Get(ctx, id)
}

// Rename changes only the display name. It may also name the default profile,
// whose initial NULL name is supplied by the frontend locale.
func (s *Store) Rename(ctx context.Context, id int64, value string) (Profile, error) {
	name, err := NormalizeName(value)
	if err != nil {
		return Profile{}, err
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE profiles
		SET name = ?, updated_at = unixepoch()
		WHERE id = ?`, name, id)
	if err != nil {
		return Profile{}, fmt.Errorf("renaming profile %d: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Profile{}, ErrNoSuchProfile
	}
	return s.Get(ctx, id)
}

// Delete removes a non-default profile and its progress through the foreign-key
// cascade. Films themselves are never touched.
func (s *Store) Delete(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var isDefault, count int
	if err := tx.QueryRowContext(ctx, `
		SELECT is_default, (SELECT COUNT(*) FROM profiles)
		FROM profiles
		WHERE id = ?`, id).Scan(&isDefault, &count); errors.Is(err, sql.ErrNoRows) {
		return ErrNoSuchProfile
	} else if err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	if isDefault != 0 {
		return ErrDefaultProfile
	}
	if count <= 1 {
		return ErrLastProfile
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("deleting profile %d: %w", id, err)
	}
	return nil
}

// SaveAvatar replaces the profile's local image and advances the version used
// by browser cache keys.
func (s *Store) SaveAvatar(ctx context.Context, id int64, mediaType string, data []byte) (Profile, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE profiles
		SET avatar_data = ?, avatar_media_type = ?,
		    avatar_version = avatar_version + 1, updated_at = unixepoch()
		WHERE id = ?`, data, mediaType, id)
	if err != nil {
		return Profile{}, fmt.Errorf("saving the avatar for profile %d: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Profile{}, ErrNoSuchProfile
	}
	return s.Get(ctx, id)
}

// Avatar returns a profile's locally stored image.
func (s *Store) Avatar(ctx context.Context, id int64) (Avatar, error) {
	var avatar Avatar
	var updated int64
	err := s.db.QueryRowContext(ctx, `
		SELECT avatar_data, avatar_media_type, avatar_version, updated_at
		FROM profiles
		WHERE id = ? AND avatar_data IS NOT NULL`,
		id,
	).Scan(&avatar.Data, &avatar.MediaType, &avatar.Version, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		if _, getErr := s.Get(ctx, id); errors.Is(getErr, ErrNoSuchProfile) {
			return Avatar{}, ErrNoSuchProfile
		}
		return Avatar{}, ErrNoAvatar
	}
	if err != nil {
		return Avatar{}, fmt.Errorf("reading the avatar for profile %d: %w", id, err)
	}
	avatar.LastUpdated = time.Unix(updated, 0).UTC()
	return avatar, nil
}

// DeleteAvatar returns a profile to the frontend's default logo.
func (s *Store) DeleteAvatar(ctx context.Context, id int64) (Profile, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE profiles
		SET avatar_data = NULL, avatar_media_type = NULL,
		    avatar_version = avatar_version + 1, updated_at = unixepoch()
		WHERE id = ?`, id)
	if err != nil {
		return Profile{}, fmt.Errorf("deleting the avatar for profile %d: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return Profile{}, ErrNoSuchProfile
	}
	return s.Get(ctx, id)
}

func scanProfile(row interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	var name sql.NullString
	var isDefault, hasAvatar int
	if err := row.Scan(&p.ID, &name, &isDefault, &hasAvatar, &p.AvatarVersion,
		&p.CreatedAt, &p.UpdatedAt); err != nil {
		return Profile{}, err
	}
	if name.Valid {
		p.Name = &name.String
	}
	p.Default = isDefault != 0
	p.HasAvatar = hasAvatar != 0
	return p, nil
}
