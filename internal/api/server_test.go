package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/profile"
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
	profiles := profile.NewStore(database)

	cfg := config.Default()
	return New(Options{
		Config:    &cfg,
		Library:   service,
		Profiles:  profiles,
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

func request(t *testing.T, h http.Handler, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	h.ServeHTTP(rec, req)
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

func TestProfilesCanBeManagedWithoutAuthentication(t *testing.T) {
	handler := newTestServer(t, bundle())

	res := get(t, handler, "/api/profiles")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200", res.StatusCode)
	}
	var list profilesResponse
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Profiles) != 1 || !list.Profiles[0].Default || list.Profiles[0].Name != nil {
		t.Fatalf("initial profiles = %+v, want one unnamed default", list.Profiles)
	}

	res = request(t, handler, http.MethodPost, "/api/profiles",
		strings.NewReader(`{"name":"Alice"}`),
		map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("create status = %d body=%s, want 201", res.StatusCode, body)
	}
	var created profile.Profile
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Name == nil || *created.Name != "Alice" {
		t.Fatalf("created = %+v, want Alice", created)
	}

	res = request(t, handler, http.MethodPatch, "/api/profiles/"+strconv.FormatInt(created.ID, 10),
		strings.NewReader(`{"name":"Alicia"}`),
		map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("rename status = %d, want 200", res.StatusCode)
	}

	res = request(t, handler, http.MethodDelete,
		"/api/profiles/"+strconv.FormatInt(list.Profiles[0].ID, 10), nil, nil)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("delete default status = %d, want 409", res.StatusCode)
	}

	res = request(t, handler, http.MethodDelete,
		"/api/profiles/"+strconv.FormatInt(created.ID, 10), nil, nil)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete created status = %d, want 204", res.StatusCode)
	}
}

func TestProfileAvatarIsVerifiedAndContentAddressedByVersion(t *testing.T) {
	handler := newTestServer(t, bundle())
	res := request(t, handler, http.MethodPost, "/api/profiles",
		strings.NewReader(`{"name":"Alice"}`),
		map[string]string{"Content-Type": "application/json"})
	var created profile.Profile
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	source := image.NewRGBA(image.Rect(0, 0, 80, 40))
	for y := range 40 {
		for x := range 80 {
			source.Set(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 6), B: 60, A: 255})
		}
	}
	var upload bytes.Buffer
	if err := png.Encode(&upload, source); err != nil {
		t.Fatal(err)
	}

	avatarPath := "/api/profiles/" + strconv.FormatInt(created.ID, 10) + "/avatar"
	res = request(t, handler, http.MethodPut, avatarPath, bytes.NewReader(upload.Bytes()),
		map[string]string{"Content-Type": "image/png"})
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("avatar upload status = %d body=%s, want 200", res.StatusCode, body)
	}
	var withAvatar profile.Profile
	if err := json.NewDecoder(res.Body).Decode(&withAvatar); err != nil {
		t.Fatal(err)
	}
	if !withAvatar.HasAvatar || withAvatar.AvatarVersion < 1 {
		t.Fatalf("profile after avatar = %+v", withAvatar)
	}

	versioned := avatarPath + "/" + strconv.FormatInt(withAvatar.AvatarVersion, 10)
	res = get(t, handler, versioned)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("avatar GET status = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	if res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("avatar response is missing nosniff")
	}
	if cache := res.Header.Get("Cache-Control"); cache != "private, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want private immutable", cache)
	}
	decoded, format, err := image.Decode(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" || decoded.Bounds().Dx() != profile.AvatarSize ||
		decoded.Bounds().Dy() != profile.AvatarSize {
		t.Errorf("served avatar = %s %v, want 512x512 JPEG", format, decoded.Bounds())
	}

	res = request(t, handler, http.MethodPut, avatarPath, bytes.NewReader(upload.Bytes()),
		map[string]string{"Content-Type": "image/png"})
	var replaced profile.Profile
	if res.StatusCode != http.StatusOK {
		t.Fatalf("replacement avatar status = %d, want 200", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(&replaced); err != nil {
		t.Fatal(err)
	}
	if replaced.AvatarVersion <= withAvatar.AvatarVersion {
		t.Fatalf("replacement avatar version = %d, want newer than %d",
			replaced.AvatarVersion, withAvatar.AvatarVersion)
	}
	if res = get(t, handler, versioned); res.StatusCode != http.StatusNotFound {
		t.Fatalf("stale avatar URL status = %d, want 404", res.StatusCode)
	}

	res = request(t, handler, http.MethodPut, avatarPath,
		strings.NewReader(`<svg onload="alert(1)"/>`),
		map[string]string{"Content-Type": "image/svg+xml"})
	if res.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("SVG upload status = %d, want 415", res.StatusCode)
	}

	res = request(t, handler, http.MethodPut, avatarPath,
		bytes.NewReader(make([]byte, maximumAvatarUpload+1)),
		map[string]string{"Content-Type": "image/png"})
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized avatar status = %d, want 413", res.StatusCode)
	}
}

func TestPersonalizedEndpointsRejectAnExplicitInvalidProfile(t *testing.T) {
	handler := newTestServer(t, bundle())

	res := request(t, handler, http.MethodGet, "/api/library/home", nil,
		map[string]string{profileHeader: "not-a-number"})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed header status = %d, want 400", res.StatusCode)
	}

	res = request(t, handler, http.MethodGet, "/api/library/home", nil,
		map[string]string{profileHeader: "999999"})
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown profile status = %d, want 404", res.StatusCode)
	}

	// No header is the compatibility contract for every v1.2 client.
	res = get(t, handler, "/api/library/home")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("missing header status = %d, want 200", res.StatusCode)
	}
	if cache := res.Header.Get("Cache-Control"); cache != "private, no-store" {
		t.Errorf("personalized cache policy = %q, want private, no-store", cache)
	}
	if vary := res.Header.Values("Vary"); len(vary) != 1 || vary[0] != profileHeader {
		t.Errorf("personalized Vary = %v, want %s", vary, profileHeader)
	}
}
