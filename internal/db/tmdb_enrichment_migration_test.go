package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// A library scanned before migration 0013 keeps every row it had, and every one
// of them comes out of the migration marked for one refresh: the new columns are
// empty and only TMDB can fill them.
func TestEnrichmentMigrationLeavesExistingRowsBehindTheCurrentFieldSet(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "before.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	if _, err := database.ExecContext(t.Context(), `
		CREATE TABLE schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}

	// Everything up to but not including the enrichment.
	names, err := migrationNames()
	if err != nil {
		t.Fatal(err)
	}
	var applied []string
	for _, name := range names {
		if name == "0013_tmdb_enrichment.sql" {
			break
		}
		applied = append(applied, name)
	}
	if len(applied) != len(names)-1 {
		t.Fatalf("0013_tmdb_enrichment.sql is not the last migration; this test needs updating")
	}
	for _, name := range applied {
		statements, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(t.Context(), database, name, string(statements)); err != nil {
			t.Fatalf("applying %s: %v", name, err)
		}
	}

	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO movies (
			path, file_name, size_bytes, modified_at, title, year,
			first_seen_scan, last_seen_scan, added_at, updated_at,
			tmdb_id, tmdb_title, overview, metadata_status, metadata_fetched_at
		) VALUES (
			'C:\Films\Arrival.mkv', 'Arrival.mkv', 1, 1, 'Arrival', 2016,
			1, 1, 100, 100,
			329865, 'Premier Contact', 'Un synopsis', 'ok', 999
		)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(t.Context(), `
		INSERT INTO series (title, year, metadata_status, metadata_fetched_at, added_at, updated_at)
		VALUES ('Severance', 2022, 'ok', 999, 100, 100)`); err != nil {
		t.Fatal(err)
	}

	// The real runner, which is what a user's upgrade actually goes through.
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	var (
		title     string
		fetchedAt int64
		version   int
		tagline   sql.NullString
	)
	if err := database.QueryRowContext(t.Context(), `
		SELECT tmdb_title, metadata_fetched_at, metadata_version, tagline
		FROM movies WHERE tmdb_id = 329865`).
		Scan(&title, &fetchedAt, &version, &tagline); err != nil {
		t.Fatal(err)
	}

	// What was already there is untouched: the migration adds columns, it does
	// not re-decide anything TMDB had already answered.
	if title != "Premier Contact" || fetchedAt != 999 {
		t.Errorf("existing metadata changed: title=%q fetched_at=%d", title, fetchedAt)
	}
	if tagline.Valid {
		t.Errorf("tagline = %q, want NULL until a refresh fills it", tagline.String)
	}
	// And it is behind: version 0 against a code that writes 2, which is what
	// makes the row stale without waiting out the ninety-day lifetime.
	if version != 0 {
		t.Errorf("metadata_version = %d, want 0 for a row written before the enrichment", version)
	}

	var seriesVersion int
	if err := database.QueryRowContext(t.Context(),
		`SELECT metadata_version FROM series WHERE title = 'Severance'`).Scan(&seriesVersion); err != nil {
		t.Fatal(err)
	}
	if seriesVersion != 0 {
		t.Errorf("series metadata_version = %d, want 0", seriesVersion)
	}

	// The index the collection row is read through has to exist, or a saga
	// lookup scans the whole library on every film page.
	var index string
	if err := database.QueryRowContext(t.Context(),
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_movies_collection'`).
		Scan(&index); err != nil {
		t.Fatalf("idx_movies_collection is missing: %v", err)
	}
}
