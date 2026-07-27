// Package library is the domain model: what a film is, how one is stored, and
// how a scan of the disk is reconciled with what is already known.
package library

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Movie is one film file, as Theia currently understands it. Everything TMDB
// will supply -- poster, synopsis, cast -- arrives in M2 and is deliberately
// absent rather than present and empty.
type Movie struct {
	ID         int64     `json:"id"`
	Path       string    `json:"path"`
	FileName   string    `json:"file_name"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`

	// Title is never empty; the parser falls back to the filename rather than
	// giving up. Year is zero when the filename did not say, which is common.
	Title string `json:"title"`
	Year  int    `json:"year,omitempty"`

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

// Upsert records a file, inserting it or refreshing what is already there. The
// path is the identity: the same path seen twice is one film, not two.
func (s *Store) Upsert(ctx context.Context, m Movie, generation int64) (upsertResult, error) {
	now := time.Now().Unix()

	var year any
	if m.Year != 0 {
		year = m.Year
	}

	// first_seen_scan is written on insert and never touched again, so comparing
	// it against the running generation says exactly which of the two branches
	// SQLite took -- no timestamp comparison, no same-second ambiguity.
	var firstSeen int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO movies
			(path, file_name, size_bytes, modified_at, title, year,
			 first_seen_scan, last_seen_scan, added_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			file_name      = excluded.file_name,
			size_bytes     = excluded.size_bytes,
			modified_at    = excluded.modified_at,
			title          = excluded.title,
			year           = excluded.year,
			last_seen_scan = excluded.last_seen_scan,
			updated_at     = excluded.updated_at
		RETURNING first_seen_scan`,
		m.Path, m.FileName, m.SizeBytes, m.ModifiedAt.Unix(), m.Title, year,
		generation, generation, now, now,
	).Scan(&firstSeen)
	if err != nil {
		return upsertResult{}, fmt.Errorf("saving %s: %w", m.Path, err)
	}
	return upsertResult{inserted: firstSeen == generation}, nil
}

// DeleteNotSeenIn removes rows the given scan did not touch, which is how a
// deleted or moved file leaves the library.
func (s *Store) DeleteNotSeenIn(ctx context.Context, generation int64) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM movies WHERE last_seen_scan < ?`, generation)
	if err != nil {
		return 0, fmt.Errorf("removing files that have disappeared: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("removing files that have disappeared: %w", err)
	}
	return int(n), nil
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
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, path, file_name, size_bytes, modified_at, title, year, added_at, updated_at
		FROM movies
		ORDER BY title COLLATE NOCASE, year
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing the library: %w", err)
	}
	defer rows.Close()

	movies := make([]Movie, 0, limit)
	for rows.Next() {
		var (
			m                              Movie
			modifiedAt, addedAt, updatedAt int64
			year                           sql.NullInt64
		)
		if err := rows.Scan(&m.ID, &m.Path, &m.FileName, &m.SizeBytes, &modifiedAt,
			&m.Title, &year, &addedAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("listing the library: %w", err)
		}
		m.ModifiedAt = time.Unix(modifiedAt, 0).UTC()
		m.AddedAt = time.Unix(addedAt, 0).UTC()
		m.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		if year.Valid {
			m.Year = int(year.Int64)
		}
		movies = append(movies, m)
	}
	return movies, rows.Err()
}
