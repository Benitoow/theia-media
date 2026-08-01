package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
)

func newMovieFileTestServer(t *testing.T) (http.Handler, *library.Service, string) {
	t.Helper()
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
	handler := New(Options{
		Config:  &cfg,
		Library: service,
		FFmpeg:  ffmpeg.New(t.TempDir(), log),
		State:   db.NewState(database),
		Web:     bundle(),
		Version: "test",
		Logger:  log,
	}).Handler()
	return handler, service, root
}

func testMediaFile(t *testing.T, root, name, contents string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMovieDetailExposesFileIDsWithoutNestedPaths(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Heat 1995 1080p.mkv", "first variant")
	testMediaFile(t, root, "Heat 1995 720p.mp4", "second variant")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, _ := service.List(t.Context(), 10, 0)

	res := get(t, handler, "/api/library/movies/"+strconvID(movies[0].ID))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	files, ok := body["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("files = %#v, want two", body["files"])
	}
	if _, present := body["path"]; present {
		t.Errorf("film detail leaked its legacy server path")
	}
	for _, raw := range files {
		file := raw.(map[string]any)
		if _, present := file["path"]; present {
			t.Errorf("nested file leaked its server path: %#v", file)
		}
		if file["id"].(float64) <= 0 || file["file_name"] == "" {
			t.Errorf("file contract is missing identity: %#v", file)
		}
	}
}

func TestSelectedAudioForcesRemuxAndReturnsStableIDs(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Heat (1995).mp4", strings.Repeat("0123456789", 20))
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), movie.ID)
	file := detail.Files[0]
	file, err := service.SaveFileMedia(t.Context(), movie.ID, file.ID, library.FileMedia{
		Status:          library.MediaOK,
		Container:       "mov,mp4,m4a,3gp,3g2,mj2",
		DurationSeconds: 100,
		Video:           &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []library.AudioTrack{
			{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true},
			{StreamIndex: 2, Codec: "aac", Language: "fre"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	trackID := file.Media.AudioTracks[1].ID

	url := "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(file.ID) +
		"/info?audio=" + strconvID(trackID)
	res := get(t, handler, url)
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d body=%s, want 200", res.StatusCode, body)
	}
	var info streamInfoResponse
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info.MovieID != movie.ID || info.FileID != file.ID || info.AudioTrackID != trackID {
		t.Errorf("ids = film %d file %d audio %d, want %d/%d/%d",
			info.MovieID, info.FileID, info.AudioTrackID, movie.ID, file.ID, trackID)
	}
	if info.Mode != string("remux") || info.ReasonCode != "audio_track_selected" {
		t.Errorf("decision = %q/%q, want remux/audio_track_selected", info.Mode, info.ReasonCode)
	}
}

func TestMovieFileRoutesEnforceOwnershipAndInspection(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Alien (1979).mkv", "alien")
	testMediaFile(t, root, "Aliens (1986).mkv", "aliens")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, _ := service.List(t.Context(), 10, 0)
	first, _ := service.Get(t.Context(), movies[0].ID)
	second, _ := service.Get(t.Context(), movies[1].ID)

	wrongOwner := "/api/stream/" + strconvID(first.ID) + "/files/" +
		strconvID(second.Files[0].ID) + "/info"
	if res := get(t, handler, wrongOwner); res.StatusCode != http.StatusNotFound || apiErrorCode(t, res) != "file_not_found" {
		t.Errorf("cross-film file response = %d, want 404 file_not_found", res.StatusCode)
	}

	pendingAudio := "/api/stream/" + strconvID(first.ID) + "/files/" +
		strconvID(first.Files[0].ID) + "/info?audio=1"
	if res := get(t, handler, pendingAudio); res.StatusCode != http.StatusConflict || apiErrorCode(t, res) != "media_not_inspected" {
		t.Errorf("uninspected audio response = %d, want 409 media_not_inspected", res.StatusCode)
	}

	invalidAudio := "/api/stream/" + strconvID(first.ID) + "/files/" +
		strconvID(first.Files[0].ID) + "/info?audio=not-an-id"
	if res := get(t, handler, invalidAudio); res.StatusCode != http.StatusBadRequest || apiErrorCode(t, res) != "invalid_audio_track_id" {
		t.Errorf("invalid audio response = %d, want 400 invalid_audio_track_id", res.StatusCode)
	}
}

func TestMovieFileDirectRouteSupportsRangesAndRejectsAudioSelection(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	contents := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 20)
	testMediaFile(t, root, "Arrival (2016).mp4", contents)
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), movie.ID)
	path := "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Range", "bytes=10-19")
	handler.ServeHTTP(recorder, request)
	res := recorder.Result()
	if res.StatusCode != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != contents[10:20] {
		t.Errorf("range body = %q, want %q", body, contents[10:20])
	}

	if res := get(t, handler, path+"?audio=1"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("direct audio-selection status = %d, want 400", res.StatusCode)
	}
}

func TestMovieFileDirectRouteReportsAFileThatDisappearedAfterScan(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	mediaPath := testMediaFile(t, root, "Gone (2012).mp4", "temporary bytes")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), movie.ID)
	if err := os.Remove(mediaPath); err != nil {
		t.Fatal(err)
	}
	path := "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID)
	for _, endpoint := range []string{path, path + "/info"} {
		res := get(t, handler, endpoint)
		if res.StatusCode != http.StatusNotFound || apiErrorCode(t, res) != "media_file_unavailable" {
			t.Errorf("missing media response for %s = %d, want 404 media_file_unavailable",
				endpoint, res.StatusCode)
		}
	}
}

func onlyAPIMovie(t *testing.T, service *library.Service) library.Movie {
	t.Helper()
	movies, err := service.List(t.Context(), 10, 0)
	if err != nil || len(movies) != 1 {
		t.Fatalf("movies = %d err=%v, want one", len(movies), err)
	}
	return movies[0]
}

func strconvID(id int64) string {
	return fmt.Sprintf("%d", id)
}

func apiErrorCode(t *testing.T, response *http.Response) string {
	t.Helper()
	var body errorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decoding API error: %v", err)
	}
	return body.Error
}
