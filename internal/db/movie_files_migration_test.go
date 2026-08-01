package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMovieFilesMigrationCopiesEveryLegacyFileWithoutChangingFilmState(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "v1.db"))
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
			tmdb_id, tmdb_title, metadata_status, metadata_fetched_at,
			duration_seconds, position_seconds, watched_at, finished
		) VALUES (
			'C:\Films\Arrival.mkv', 'Arrival.mkv', 123456, 111, 'Arrival', 2016,
			3, 7, 100, 200,
			329865, 'Arrival', 'ok', 150,
			6960, 1234, 175, 0
		)`)
	if err != nil {
		t.Fatal(err)
	}
	movieID, _ := result.LastInsertId()

	if err := Migrate(t.Context(), database); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("second migration pass: %v", err)
	}

	var (
		fileMovieID, size, modified, firstSeen, lastSeen int64
		path, name, status                               string
		primary                                          int
		duration                                         float64
	)
	err = database.QueryRowContext(t.Context(), `
		SELECT movie_id, path, file_name, size_bytes, modified_at,
		       first_seen_scan, last_seen_scan, is_primary,
		       media_status, media_duration_seconds
		FROM movie_files`).Scan(
		&fileMovieID, &path, &name, &size, &modified,
		&firstSeen, &lastSeen, &primary, &status, &duration,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fileMovieID != movieID || path != `C:\Films\Arrival.mkv` || name != "Arrival.mkv" {
		t.Errorf("migrated file = film %d %q %q, want film %d and the original path",
			fileMovieID, path, name, movieID)
	}
	if size != 123456 || modified != 111 || firstSeen != 3 || lastSeen != 7 || primary != 1 {
		t.Errorf("migrated file fields were not preserved")
	}
	if status != "pending" || duration != 6960 {
		t.Errorf("media = %q duration %v, want pending with known duration 6960", status, duration)
	}

	var tmdbID int
	var position float64
	if err := database.QueryRowContext(t.Context(), `
		SELECT tmdb_id, position_seconds FROM movies WHERE id = ?`, movieID,
	).Scan(&tmdbID, &position); err != nil {
		t.Fatal(err)
	}
	if tmdbID != 329865 || position != 1234 {
		t.Errorf("film state = TMDB %d progress %v, want 329865 and 1234", tmdbID, position)
	}
}
