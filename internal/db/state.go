package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Keys used in the app_state table. Constants so a typo is a compile error
// rather than a value that silently reads back empty forever.
const (
	// KeyOnboardingCompleted is set once the welcome screen has been dismissed.
	KeyOnboardingCompleted = "onboarding_completed_at"
)

// State reads and writes the application's own remembered values, as opposed to
// the user's configuration.
type State struct {
	db *sql.DB
}

// NewState wraps an already-migrated database.
func NewState(database *sql.DB) *State {
	return &State{db: database}
}

// Get returns a value and whether it was present.
func (s *State) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_state WHERE key = ?`, key).Scan(&value)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("reading app state %q: %w", key, err)
	}
	return value, true, nil
}

// Set stores a value, replacing any previous one.
func (s *State) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_state (key, value, updated_at)
		VALUES (?, ?, unixepoch())
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value)
	if err != nil {
		return fmt.Errorf("writing app state %q: %w", key, err)
	}
	return nil
}

// Has reports whether a key is present.
func (s *State) Has(ctx context.Context, key string) (bool, error) {
	_, ok, err := s.Get(ctx, key)
	return ok, err
}
