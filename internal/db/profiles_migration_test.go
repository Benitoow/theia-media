package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestProfilesMigrationPreservesProgressAndRollbackBridge(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "v4.db"))
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

	if err := Migrate(t.Context(), database); err != nil {
		t.Fatal(err)
	}
	// A second pass must be a no-op, as it is on every normal startup.
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("second migration pass: %v", err)
	}

	var profileID int64
	var name sql.NullString
	var isDefault int
	if err := database.QueryRowContext(t.Context(),
		`SELECT id, name, is_default FROM profiles`).Scan(&profileID, &name, &isDefault); err != nil {
		t.Fatal(err)
	}
	if profileID != 1 || name.Valid || isDefault != 1 {
		t.Errorf("migrated profile = id %d name %v default %d, want unnamed default id 1",
			profileID, name, isDefault)
	}

	var position, duration float64
	if err := database.QueryRowContext(t.Context(), `
		SELECT p.position_seconds, m.duration_seconds
		FROM playback_progress AS p
		JOIN movies AS m ON m.id = p.movie_id
		WHERE p.profile_id = 1 AND p.movie_id = ?`, movieID).Scan(&position, &duration); err != nil {
		t.Fatal(err)
	}
	if position != 1234 || duration != 6960 {
		t.Errorf("migrated position/duration = %v/%v, want 1234/6960", position, duration)
	}

	// Simulate v1.2 after an executable rollback. Its legacy UPDATE must be
	// visible again to the profile-aware binary on the next start.
	if _, err := database.ExecContext(t.Context(), `
		UPDATE movies
		SET position_seconds = 2345, watched_at = 30, finished = 0
		WHERE id = ?`, movieID); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(t.Context(), `
		SELECT position_seconds
		FROM playback_progress
		WHERE profile_id = 1 AND movie_id = ?`, movieID).Scan(&position); err != nil {
		t.Fatal(err)
	}
	if position != 2345 {
		t.Errorf("position after legacy write = %v, want 2345", position)
	}

	rows, err := database.QueryContext(t.Context(), `PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Error("profiles migration left a foreign-key violation")
	}
}
