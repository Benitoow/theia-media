package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/library"
)

func writeEpisodeMedia(t *testing.T, root, relative, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func episodeFixture(t *testing.T) (http.Handler, *library.Service, library.Series, []library.EpisodeItem) {
	t.Helper()
	handler, service, root := newMovieFileTestServer(t)
	writeEpisodeMedia(t, root, "Shows/Severance (2022)/Season 01/S01E01.720p.mp4",
		strings.Repeat("abcdefghijklmnopqrstuvwxyz", 20))
	writeEpisodeMedia(t, root, "Shows/Severance (2022)/Season 01/S01E02E03.mkv",
		strings.Repeat("0123456789", 50))
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	seriesList, err := service.ListSeries(t.Context(), 10, 0)
	if err != nil || len(seriesList) != 1 {
		t.Fatalf("series = %+v, err = %v", seriesList, err)
	}
	season, err := service.GetSeason(t.Context(), defaultProfileID, seriesList[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	return handler, service, seriesList[0], season.Items
}

func TestSeriesCatalogueDetailSeasonAndEpisodeContracts(t *testing.T) {
	handler, _, series, items := episodeFixture(t)

	for _, endpoint := range []string{
		"/api/library/series",
		"/api/library/series/" + strconvID(series.ID),
		"/api/library/series/" + strconvID(series.ID) + "/seasons/1",
		"/api/library/episodes/" + strconvID(items[0].ID),
	} {
		res := get(t, handler, endpoint)
		if res.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(res.Body)
			t.Fatalf("GET %s = %d %s", endpoint, res.StatusCode, body)
		}
	}

	res := get(t, handler, "/api/library/episodes/"+strconvID(items[0].ID))
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["kind"] != "episode" || body["series_title"] != "Severance" {
		t.Fatalf("episode identity = %#v", body)
	}
	files := body["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("files = %#v", files)
	}
	if _, leaked := files[0].(map[string]any)["path"]; leaked {
		t.Fatal("episode detail leaked a server path")
	}

	stats := get(t, handler, "/api/library/stats")
	var counts statsResponse
	if err := json.NewDecoder(stats.Body).Decode(&counts); err != nil {
		t.Fatal(err)
	}
	if counts.Movies != 0 || counts.Series != 1 || counts.Episodes != 2 {
		t.Fatalf("stats = %+v", counts)
	}
}

func TestEpisodeProgressAndSeriesHome(t *testing.T) {
	handler, _, _, items := episodeFixture(t)
	path := "/api/library/episodes/" + strconvID(items[0].ID) + "/progress"
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, path,
		bytes.NewBufferString(`{"position_seconds":180,"duration_seconds":1200}`))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save progress = %d %s", recorder.Code, recorder.Body.String())
	}

	home := get(t, handler, "/api/library/series/home")
	if home.StatusCode != http.StatusOK {
		t.Fatalf("series home = %d", home.StatusCode)
	}
	var payload library.SeriesHome
	if err := json.NewDecoder(home.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Continue) != 1 || payload.Continue[0].ID != items[0].ID {
		t.Fatalf("continue watching = %+v", payload.Continue)
	}

	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, httptest.NewRequest(http.MethodDelete, path, nil))
	if reset.Code != http.StatusNoContent {
		t.Fatalf("reset = %d", reset.Code)
	}
}

func TestEpisodeSelectedAudioForcesRemuxAndOwnershipIsEnforced(t *testing.T) {
	handler, service, _, items := episodeFixture(t)
	first, err := service.GetEpisodeItem(t.Context(), defaultProfileID, items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.SaveEpisodeFileMedia(t.Context(), first.ID, first.Files[0].ID, library.FileMedia{
		Status:          library.MediaOK,
		Container:       "mp4",
		DurationSeconds: 1200,
		Video:           &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1280, Height: 720},
		AudioTracks: []library.AudioTrack{
			{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true},
			{StreamIndex: 2, Codec: "aac", Language: "fra"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	trackID := file.Media.AudioTracks[1].ID
	base := "/api/library/episodes/" + strconvID(first.ID) + "/files/" + strconvID(file.ID) + "/stream"
	res := get(t, handler, base+"/info?audio="+strconvID(trackID))
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("stream info = %d %s", res.StatusCode, body)
	}
	var info streamInfoResponse
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if info.EpisodeID != first.ID || info.FileID != file.ID || info.AudioTrackID != trackID ||
		info.Mode != "remux" || info.ReasonCode != "audio_track_selected" {
		t.Fatalf("stream decision = %+v", info)
	}

	second, err := service.GetEpisodeItem(t.Context(), defaultProfileID, items[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	wrong := "/api/library/episodes/" + strconvID(second.ID) + "/files/" + strconvID(file.ID) + "/stream/info"
	if res := get(t, handler, wrong); res.StatusCode != http.StatusNotFound || apiErrorCode(t, res) != "file_not_found" {
		t.Fatalf("cross-episode file = %d, want file_not_found", res.StatusCode)
	}
}

func TestEpisodeDirectStreamSupportsRanges(t *testing.T) {
	handler, service, _, items := episodeFixture(t)
	item, err := service.GetEpisodeItem(t.Context(), defaultProfileID, items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/library/episodes/" + strconvID(item.ID) + "/files/" +
		strconvID(item.Files[0].ID) + "/stream"
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Range", "bytes=10-19")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", recorder.Code)
	}
	if recorder.Body.String() != "klmnopqrst" {
		t.Fatalf("range body = %q", recorder.Body.String())
	}
	if res := get(t, handler, path+"?audio=1"); res.StatusCode != http.StatusBadRequest ||
		apiErrorCode(t, res) != "audio_selection_requires_remux" {
		t.Fatalf("direct audio selection = %d", res.StatusCode)
	}
}

// defaultProfileID is the profile migration 0009 creates.
const defaultProfileID int64 = 1
