package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOriginalSeriesLanguageUpgradePreservesHistoryAndRefreshesOnlyMatchedSeries(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "before.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	names, err := migrationNames()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE schema_migrations(name TEXT PRIMARY KEY,applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if name == "0017_series_original_language.sql" {
			break
		}
		statements, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(t.Context(), database, name, string(statements)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(`INSERT INTO series(id,title,metadata_status,metadata_fetched_at,added_at,updated_at) VALUES(7,'Matched','ok',999,1,1),(8,'Unmatched','not_found',888,1,1);
 INSERT INTO seasons(id,series_id,season_number,added_at,updated_at) VALUES(7,7,1,1,1);
 INSERT INTO episode_items(id,season_id,episode_key,first_episode,last_episode,added_at,updated_at) VALUES(7,7,'1',1,1,1,1);
 INSERT INTO episode_progress(profile_id,episode_item_id,position_seconds,watched_at,finished) VALUES(1,7,420,999,0);`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatal(err)
	}
	var language sql.NullString
	var fetched int64
	if err := database.QueryRow(`SELECT original_language,metadata_fetched_at FROM series WHERE id=7`).Scan(&language, &fetched); err != nil {
		t.Fatal(err)
	}
	if language.Valid || fetched != 0 {
		t.Fatalf("backfill state=%+v %d", language, fetched)
	}
	if err := database.QueryRow(`SELECT metadata_fetched_at FROM series WHERE id=8`).Scan(&fetched); err != nil || fetched != 888 {
		t.Fatalf("unmatched lookup was reset: %d %v", fetched, err)
	}
	var position float64
	if err := database.QueryRow(`SELECT position_seconds FROM episode_progress WHERE profile_id=1 AND episode_item_id=7`).Scan(&position); err != nil || position != 420 {
		t.Fatalf("history changed: %v %v", position, err)
	}
	if err := Migrate(t.Context(), database); err != nil {
		t.Fatal(err)
	}
}
