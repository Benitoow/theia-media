package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/library"
)

// bundle stands in for a real SvelteKit build: an entry point, a hashed asset
// and a file from static/.
func bundle() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                           {Data: []byte("<!doctype html><title>Theia</title>")},
		"favicon.svg":                          {Data: []byte("<svg/>")},
		"_app/immutable/chunks/abc123.js":      {Data: []byte("export default 1")},
		"_app/immutable/assets/0.deadbeef.css": {Data: []byte("body{}")},
	}
}

func newTestServer(t *testing.T, web fstest.MapFS) http.Handler {
	t.Helper()
	handler, _ := newTestServerWithLibrary(t, web)
	return handler
}

// newTestServerWithLibrary builds a server backed by a real, migrated database
// in a temporary directory. An in-memory SQLite would need shared-cache games
// to survive database/sql's connection pool; a temp file is simpler and closer
// to what actually runs.
func newTestServerWithLibrary(t *testing.T, web fstest.MapFS) (http.Handler, *library.Service) {
	t.Helper()

	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	log := slog.New(slog.DiscardHandler)
	service := library.NewService(library.NewStore(database), nil, log)

	cfg := config.Default()
	return New(Options{
		Config:    &cfg,
		Library:   service,
		State:     db.NewState(database),
		Web:       web,
		Version:   "test-version",
		KeySource: config.KeyMissing,
		Logger:    log,
	}).Handler(), service
}

func get(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

func TestHealthReportsTheRunningVersion(t *testing.T) {
	res := get(t, newTestServer(t, bundle()), "/api/health")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}

	var body healthResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.Version != "test-version" {
		t.Errorf("version = %q, want %q", body.Version, "test-version")
	}
}

func TestUnknownAPIPathIs404JSON(t *testing.T) {
	// The frontend parses every /api/ response as JSON. Falling back to the SPA
	// here would hand it HTML and produce a parse error instead of a 404.
	res := get(t, newTestServer(t, bundle()), "/api/does-not-exist")

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
}

func TestUnknownPathFallsBackToTheSPA(t *testing.T) {
	// A deep link such as /films/42 is a client-side route: it has to return
	// index.html so the router can pick it up, not a 404.
	res := get(t, newTestServer(t, bundle()), "/films/42")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "<!doctype html><title>Theia</title>" {
		t.Errorf("body = %q, want index.html", body)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache: a stale index.html points at bundles that no longer exist", cc)
	}
}

func TestHashedAssetsAreCachedForever(t *testing.T) {
	res := get(t, newTestServer(t, bundle()), "/_app/immutable/chunks/abc123.js")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want the immutable policy", cc)
	}
}

func TestOrdinaryAssetsAreNotCachedForever(t *testing.T) {
	// favicon.svg keeps its name across builds, so caching it for a year would
	// pin the old one until the browser cache is cleared by hand.
	res := get(t, newTestServer(t, bundle()), "/favicon.svg")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if cc := res.Header.Get("Cache-Control"); cc == "public, max-age=31536000, immutable" {
		t.Error("favicon.svg was served with the immutable cache policy")
	}
}

func TestMissingFrontendServesThePlaceholder(t *testing.T) {
	// This is what you get running the binary without having built web/ first.
	// It has to be an obvious diagnostic, not a blank page or a crash.
	res := get(t, newTestServer(t, fstest.MapFS{}), "/")

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if len(body) == 0 {
		t.Error("the placeholder page is empty")
	}
}

func TestAPIStillWorksWithoutAFrontend(t *testing.T) {
	// A missing bundle must not take the API down with it -- that is how you
	// diagnose the missing bundle in the first place.
	res := get(t, newTestServer(t, fstest.MapFS{}), "/api/health")

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

func TestDirectoryPathsDoNotBypassTheSPAFallback(t *testing.T) {
	// fs.Stat succeeds for directories, so serving them would send the file
	// server's directory listing instead of the app.
	res := get(t, newTestServer(t, bundle()), "/_app/immutable")

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "<!doctype html><title>Theia</title>" {
		t.Errorf("status = %d body = %q, want index.html", res.StatusCode, body)
	}
}
