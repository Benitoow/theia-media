package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/fakeffmpeg"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/playback"
	"github.com/Benitoow/theia-media/internal/profiles"
)

// TestMain lets this test binary double as a fake ffmpeg for the tests that
// hold a live converted stream. See the fakeffmpeg package: in a normal run
// this returns immediately.
func TestMain(m *testing.M) {
	fakeffmpeg.MaybeRun()
	os.Exit(m.Run())
}

// TestConvertedStreamCeilingRefusalRidesTheRealRoute drives the ceiling and
// its refusal through the film remux route itself, with the server wired to a
// fake binary. Under test is the adapter seam: the refusal's exact shape the
// transport retries (503, transcode_busy, Retry-After), and the kill that
// shutdown calls on this server.
func TestConvertedStreamCeilingRefusalRidesTheRealRoute(t *testing.T) {
	markers := t.TempDir()
	restore := fakeffmpeg.SetMode(fakeffmpeg.ModeLive, markers)
	defer restore()

	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	root := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	service := library.NewService(library.NewStore(database), nil, log)
	cfg := config.Default()
	cfg.LibraryPaths = []string{root}
	// The Manager stays real and empty: nothing in this route touches it once
	// the media is measured, which is what keeps the test from downloading.
	apiServer := New(Options{
		Config:   &cfg,
		Library:  service,
		FFmpeg:   ffmpeg.New(t.TempDir(), log),
		State:    db.NewState(database),
		Profiles: profiles.New(database),
		Playback: playback.NewService(playback.Options{
			Binary:      &fakeffmpeg.Binary{},
			StreamLimit: 1,
			Logger:      log,
		}),
		Web:     bundle(),
		Version: "test",
		Logger:  log,
	})
	ts := httptest.NewServer(apiServer.Handler())
	defer ts.Close()

	if err := os.WriteFile(filepath.Join(root, "Heat (1995).mkv"), []byte("an mkv the browser needs rewrapped"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
	file, err := service.SaveFileMedia(t.Context(), movie.ID, detail.Files[0].ID, library.FileMedia{
		Status:           library.MediaOK,
		Container:        "matroska,webm",
		DurationSeconds:  5400,
		Video:            &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks:      []library.AudioTrack{{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true}},
		SubtitlesScanned: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	remuxURL := ts.URL + "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(file.ID) + "/remux"

	// The first viewer holds the only remux slot, mid-stream.
	held, err := ts.Client().Get(remuxURL)
	if err != nil {
		t.Fatalf("the held remux failed: %v", err)
	}
	defer held.Body.Close()
	if held.StatusCode != http.StatusOK {
		t.Fatalf("the held remux = %d, want 200", held.StatusCode)
	}

	// The second is refused with the shape the transport retries.
	res, err := ts.Client().Get(remuxURL)
	if err != nil {
		t.Fatalf("the refused remux failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the ceiling refusal = %d, want 503", res.StatusCode)
	}
	if retry := res.Header.Get("Retry-After"); retry != "1" {
		t.Errorf("retry-after = %q, want 1", retry)
	}
	var refusal struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&refusal); err != nil {
		t.Fatalf("decoding the refusal: %v", err)
	}
	if refusal.Error != "transcode_busy" {
		t.Errorf("code = %q, want transcode_busy", refusal.Error)
	}

	// Shutdown kills what is live: the held stream's pipe closes and its
	// viewer gets the bytes that were already on the way.
	apiServer.KillStreams()
	body, readErr := io.ReadAll(held.Body)
	if len(body) < 1024 {
		t.Errorf("the held stream delivered %d bytes, want at least the first chunk", len(body))
	}
	if readErr != nil {
		// A killed encoder ends the body abruptly; a clean EOF is equally
		// acceptable, and neither is a hang.
		t.Logf("the killed stream ended with %v", readErr)
	}
}
