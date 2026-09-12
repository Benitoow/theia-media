package recovery

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
)

func TestRollbackRestoresDatabaseConfigAndExecutable(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(context.Background(), filepath.Join(dir, db.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE before_update (value TEXT); INSERT INTO before_update VALUES ('kept')`); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"port":8383}`), 0o600); err != nil {
		t.Fatal(err)
	}
	execPath := filepath.Join(dir, "theia.exe")
	if err := os.WriteFile(execPath, []byte("old executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.DiscardHandler)
	old := New(dir, execPath, "1.0.0", log)
	if err := old.Prepare(context.Background(), database, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE after_update (value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"port":9999}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(execPath, execPath+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(execPath, []byte("new executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	current := New(dir, execPath, "2.0.0", log)
	if !current.PendingFor("2.0.0") {
		t.Fatal("recovery point was not recognised by the target version")
	}
	if err := current.Rollback(); err != nil {
		t.Fatal(err)
	}

	gotExecutable, _ := os.ReadFile(execPath)
	if string(gotExecutable) != "old executable" {
		t.Fatalf("executable = %q", gotExecutable)
	}
	gotConfig, _ := os.ReadFile(configPath)
	if string(gotConfig) != `{"port":8383}` {
		t.Fatalf("config = %q", gotConfig)
	}
	restored, err := db.Open(context.Background(), filepath.Join(dir, db.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var value string
	if err := restored.QueryRow(`SELECT value FROM before_update`).Scan(&value); err != nil || value != "kept" {
		t.Fatalf("snapshot data = %q, err = %v", value, err)
	}
	if _, err := restored.Exec(`SELECT * FROM after_update`); err == nil {
		t.Fatal("post-snapshot schema survived rollback")
	}
}
