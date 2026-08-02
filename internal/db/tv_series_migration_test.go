package db

import (
	"path/filepath"
	"testing"
)

func TestTVSeriesMigrationIsAdditiveAndIdempotent(t *testing.T) {
	database, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatalf("second migration pass: %v", err)
	}

	wantTables := []string{
		"movies", "movie_files",
		"series", "seasons", "episodes", "episode_items",
		"episode_item_members", "episode_files", "episode_file_audio_tracks",
	}
	for _, table := range wantTables {
		var count int
		if err := database.QueryRowContext(t.Context(), `
			SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %q count = %d, want 1", table, count)
		}
	}

	var applied int
	if err := database.QueryRowContext(t.Context(), `
		SELECT COUNT(*) FROM schema_migrations WHERE name = '0007_tv_series.sql'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("migration record count = %d, want 1", applied)
	}
}
