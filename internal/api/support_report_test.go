package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/supportlog"
)

func supportTestServer(t *testing.T) (http.Handler, *supportlog.Store) {
	t.Helper()
	logger, store, err := supportlog.New(t.TempDir(), io.Discard, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	cfg := config.Default()
	cfg.LibraryPaths = []string{`C:\Users\Alice\Private Videos`}
	logger.Info("fixture with private values",
		"tmdb_api_key", "fixture-secret-value",
		"path", `C:\Users\Alice\Private Videos\Dune.mkv`)
	return New(Options{
		Config:      &cfg,
		Web:         bundle(),
		Version:     "3.2.0-preview.test",
		Logger:      logger,
		SupportLogs: store,
	}).Handler(), store
}

func postJSON(t *testing.T, handler http.Handler, path, body string) *http.Response {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func TestSupportExportContainsCorrelatedEventsWithoutPrivateValues(t *testing.T) {
	handler, _ := supportTestServer(t)
	event := `{
		"event":"playback_waiting",
		"client":{"user_agent":"Theia Test Browser","hardware_concurrency":8},
		"playback":{"session_id":"session-1","item_kind":"movie","item_id":42,"file_id":7,"buffered_seconds":0.5}
	}`
	if response := postJSON(t, handler, "/api/diagnostics/events", event); response.StatusCode != http.StatusNoContent {
		t.Fatalf("event status = %d, want 204", response.StatusCode)
	}

	response := postJSON(t, handler, "/api/diagnostics/export", `{"client":{"platform":"Test OS"}}`)
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("export status = %d body=%s, want 200", response.StatusCode, body)
	}
	if response.Header.Get("Content-Type") != "application/zip" || !strings.Contains(response.Header.Get("Content-Disposition"), "theia-support-") {
		t.Fatalf("export headers = %#v", response.Header)
	}
	archiveBytes, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		contents[file.Name] = string(data)
	}
	if !strings.Contains(contents["README.txt"], "Version: 3.2.0-preview.test") {
		t.Errorf("README is missing the running version: %s", contents["README.txt"])
	}
	if _, ok := contents["diagnostics.json"]; !ok {
		t.Fatal("diagnostics.json is absent")
	}
	var diagnostics map[string]any
	if err := json.Unmarshal([]byte(contents["diagnostics.json"]), &diagnostics); err != nil {
		t.Fatalf("sanitized diagnostics are not valid JSON: %v", err)
	}
	all := strings.Join(mapValues(contents), "\n")
	for _, private := range []string{"fixture-secret-value", `C:\Users\Alice\Private Videos`, "Dune.mkv"} {
		if strings.Contains(all, private) {
			t.Errorf("support archive leaked %q", private)
		}
	}
	if !strings.Contains(all, "<LIBRARY_1>") {
		t.Error("support archive did not replace the library path")
	}
	if !strings.Contains(all, "playback_waiting") || !strings.Contains(all, `"item_id":42`) {
		t.Error("support archive is missing the correlated playback event")
	}
}

func TestUnknownClientDiagnosticIsRejected(t *testing.T) {
	handler, _ := supportTestServer(t)
	response := postJSON(t, handler, "/api/diagnostics/events", `{"event":"write_anything"}`)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
