// Package library is the domain model: what a film is, how one is stored, and
// how a scan of the disk is reconciled with what is already known.
package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Movie is one film people browse. The legacy file fields identify its primary
// file and remain in the JSON contract until the existing frontend has moved to
// Files. The database source of truth for playable files is movie_files.
type Movie struct {
	ID         int64     `json:"id"`
	Path       string    `json:"-"`
	FileName   string    `json:"file_name"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`

	// Title is never empty; the parser falls back to the filename rather than
	// giving up. Year is zero when the filename did not say, which is common.
	// Both come from the filename and stay as they are even once TMDB has
	// something better to say -- the interface prefers Metadata.Title and falls
	// back to these, so a film TMDB never recognised still has a name.
	Title string `json:"title"`
	Year  int    `json:"year,omitempty"`

	Metadata Metadata    `json:"metadata"`
	Progress Progress    `json:"progress"`
	Files    []MovieFile `json:"files,omitempty"`

	AddedAt   time.Time `json:"added_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store is the persistence layer for the library.
type Store struct {
	db *sql.DB
}

// NewStore wraps an already-migrated database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// NextScanGeneration bumps the scan counter and returns the new value. Every
// scan calls this exactly once and stamps every row it touches with the result.
func (s *Store) NextScanGeneration(ctx context.Context) (int64, error) {
	var generation int64
	err := s.db.QueryRowContext(ctx,
		`UPDATE scan_generation SET value = value + 1 WHERE id = 1 RETURNING value`,
	).Scan(&generation)
	if err != nil {
		return 0, fmt.Errorf("starting a scan: %w", err)
	}
	return generation, nil
}

// upsertResult reports what a single Upsert did, so a scan can tell the user
// how much actually changed rather than just how many files exist.
type upsertResult struct {
	inserted bool
}

// Upsert records a file, inserting it or refreshing what is already there.
// movie_files owns path identity; conservative association decides whether a
// new path belongs under an existing film.
func (s *Store) Upsert(ctx context.Context, m Movie, generation int64) (upsertResult, error) {
	return s.upsertFile(ctx, m, generation)
}

// DeleteNotSeenIn removes rows the given scan did not touch, which is how a
// deleted or moved file leaves the library.
func (s *Store) DeleteNotSeenIn(ctx context.Context, generation int64) (int, error) {
	return s.deleteFilesNotSeenIn(ctx, generation)
}

// Count returns how many films are in the library.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM movies`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting the library: %w", err)
	}
	return n, nil
}

// List returns films ordered by title. The ordering is NOCASE so that "the
// matrix" and "The Matrix" do not end up in different halves of the list.
func (s *Store) List(ctx context.Context, limit, offset int) ([]Movie, error) {
	// Asking for more than the ceiling gets you the ceiling. An earlier version
	// fell back to the default here, so limit=600 quietly returned 100 rows and
	// the caller had no way to tell a short page from the end of the library.
	const maxLimit = 500
	switch {
	case limit <= 0:
		limit = 100
	case limit > maxLimit:
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		FROM movies
		ORDER BY title COLLATE NOCASE, year
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing the library: %w", err)
	}
	movies, err := collectMovies(rows, limit)
	if err != nil {
		return nil, fmt.Errorf("listing the library: %w", err)
	}
	return movies, nil
}

// unix converts a stored timestamp, leaving the zero value alone so that
// "never" does not become 1970 on its way to the interface.
func unix(seconds int64) time.Time {
	if seconds == 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}
