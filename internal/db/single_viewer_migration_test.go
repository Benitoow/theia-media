package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSingleViewerMigrationRemovesRetiredViewerDataAndKeepsMovieProgress(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "v5.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	if _, err := database.ExecContext(t.Context(), `
		PRAGMA foreign_keys = ON;
		CREATE TABLE schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"0001_initial.sql",
		"0002_tmdb_metadata.sql",
		"0003_playback_progress.sql",
		"0004_app_state.sql",
	} {
		statements, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(t.Context(), database, name, string(statements)); err != nil {
			t.Fatal(err)
		}
	}

	result, err := database.ExecContext(t.Context(), `
		INSERT INTO movies (
			path, file_name, size_bytes, modified_at, title, year,
			first_seen_scan, last_seen_scan, added_at, updated_at,
			duration_seconds, position_seconds, watched_at, finished
		) VALUES (
			'C:\Films\Arrival.mkv', 'Arrival.mkv', 1000, 10, 'Arrival', 2016,
			1, 1, 10, 10, 6960, 1234, 20, 0
		)`)
	if err != nil {
		t.Fatal(err)
	}
	movieID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	// Recreate the relevant v1.5 schema. The old migration is intentionally no
	// longer embedded, but existing installations still carry these objects.
	if _, err := database.ExecContext(t.Context(), `
		CREATE TABLE profiles (
			id INTEGER PRIMARY KEY,
			name TEXT,
			is_default INTEGER NOT NULL,
			avatar_data BLOB
		);
		CREATE TABLE playback_progress (
			profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			movie_id INTEGER NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
			position_seconds REAL NOT NULL,
			watched_at INTEGER NOT NULL,
			finished INTEGER NOT NULL,
			PRIMARY KEY (profile_id, movie_id)
		);
		CREATE TRIGGER sync_legacy_progress_to_default
		AFTER UPDATE OF position_seconds ON movies
		BEGIN
			UPDATE playback_progress
			SET position_seconds = NEW.position_seconds
			WHERE profile_id = 1 AND movie_id = NEW.id;
		END;
		INSERT INTO profiles (id, name, is_default) VALUES
			(1, NULL, 1), (2, 'Guest', 0);
		INSERT INTO playback_progress
			(profile_id, movie_id, position_seconds, watched_at, finished)
		VALUES
			(1, ?, 1234, 20, 0), (2, ?, 4321, 30, 0);
		INSERT INTO schema_migrations (name, applied_at)
		VALUES ('0005_profiles.sql', unixepoch());
	`, movieID, movieID); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(t.Context(), database); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("second migration pass: %v", err)
	}

	var position, duration float64
	if err := database.QueryRowContext(t.Context(), `
		SELECT position_seconds, duration_seconds FROM movies WHERE id = ?`, movieID,
	).Scan(&position, &duration); err != nil {
		t.Fatal(err)
	}
	if position != 1234 || duration != 6960 {
		t.Errorf("movie progress = %v/%v, want 1234/6960", position, duration)
	}

	// `profiles` is deliberately not in this list any more. 0009 creates a table
	// of that name, but it is a new one: what 0005 had to destroy was the old
	// model -- the separate progress table and the trigger that mirrored it --
	// and those must stay gone. Decision 48 rebuilt profiles from scratch rather
	// than reviving this schema, so the check is now about shape, not naming.
	for _, object := range []string{"playback_progress", "sync_legacy_progress_to_default"} {
		var count int
		if err := database.QueryRowContext(t.Context(), `
			SELECT COUNT(*) FROM sqlite_schema WHERE name = ?`, object,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("retired database object %q still exists", object)
		}
	}

	// The retired avatar column is the clearest proof the old table did not come
	// back under its own name.
	var retiredColumns int
	if err := database.QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM pragma_table_info('profiles') WHERE name = 'avatar'`,
	).Scan(&retiredColumns); err != nil {
		t.Fatal(err)
	}
	if retiredColumns != 0 {
		t.Errorf("the retired profiles schema was resurrected rather than rebuilt")
	}

	var historicalMigrationCount int
	if err := database.QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM schema_migrations WHERE name = '0005_profiles.sql'`,
	).Scan(&historicalMigrationCount); err != nil {
		t.Fatal(err)
	}
	if historicalMigrationCount != 1 {
		t.Errorf("historical migration marker count = %d, want 1", historicalMigrationCount)
	}
}
