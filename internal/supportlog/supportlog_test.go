package supportlog

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestFileHistoryKeepsDebugRecordsAndRedactsSecrets(t *testing.T) {
	var console bytes.Buffer
	logger, store, err := New(t.TempDir(), &console, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	logger.Debug("decoder sample", "frames", 42)
	logger.Info("configured", "tmdb_api_key", "not-a-real-secret")
	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	joined := string(files[len(files)-1].Data)
	if !strings.Contains(joined, "decoder sample") {
		t.Error("debug record is absent from the persistent history")
	}
	if strings.Contains(joined, "not-a-real-secret") || !strings.Contains(joined, "[REDACTED]") {
		t.Errorf("persistent record did not redact the secret: %s", joined)
	}
	if strings.Contains(console.String(), "decoder sample") {
		t.Error("debug record leaked into the non-verbose console")
	}
}

func TestStoreRotatesCompleteRecordsOldestFirst(t *testing.T) {
	store, err := openStore(t.TempDir(), 12, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	for _, line := range []string{"first-line\n", "second-line\n", "third-line\n"} {
		if _, err := store.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files = %d, want active plus two backups", len(files))
	}
	if string(files[0].Data) != "first-line\n" || string(files[2].Data) != "third-line\n" {
		t.Errorf("rotation order/content = %#v", files)
	}
}
