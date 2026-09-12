package api

// Golden playback-contract tests. They pin the observable HTTP behaviour the
// backend found in when docs/plan-refonte-lecture.md (tranche 1) started, so
// the extraction that follows can be judged on responses that did not move.
//
// Everything here runs without an ffmpeg binary on disk: the /info, seek and
// subtitle routes must work (the M1 promise -- asking a question never
// downloads ffmpeg), and the remux routes are pinned at their first guard.
// A converted stream that actually produces bytes is covered by the browser
// playback suite against a real ffmpeg, and the fake-binary harness needed to
// pin its 200/502 paths at unit level arrives with tranche 2's byte
// comparison, not before.

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/library"
	"github.com/Benitoow/theia-media/internal/profiles"
)

// newPlaybackGuardServer is the movie-file test server with no FFmpeg manager
// at all. No unit test may reach Manager.Path: on a supported platform it
// downloads, and the remux guards have to be pinned before that gate anyway.
func newPlaybackGuardServer(t *testing.T) (http.Handler, *library.Service, string) {
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
		Config:   &cfg,
		Library:  service,
		FFmpeg:   nil,
		State:    db.NewState(database),
		Profiles: profiles.New(database),
		Web:      bundle(),
		Version:  "test",
		Logger:   log,
	}).Handler()
	return handler, service, root
}

func decodeStreamInfo(t *testing.T, res *http.Response) streamInfoResponse {
	t.Helper()
	var info streamInfoResponse
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		t.Fatalf("decoding stream info: %v", err)
	}
	return info
}

// TestStreamInfoUnmeasuredMediaDecidesByContainerOnly pins the decision the
// server can make without running anything: an MP4 is handed over and an MKV
// is remuxed, whatever their streams turn out to be. The MP4 blind spot is
// documented in the stream package and stays as found; tranche 2 must keep
// this answer byte-identical.
func TestStreamInfoUnmeasuredMediaDecidesByContainerOnly(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Rush Hour (1998).mp4", "an mp4 the browser opens")
	testMediaFile(t, root, "Heat (1995).mkv", "an mkv that needs rewrapping")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil || len(movies) != 2 {
		t.Fatalf("movies = %d, err = %v, want two", len(movies), err)
	}

	for _, movie := range movies {
		detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
		res := get(t, handler, "/api/stream/"+strconvID(movie.ID)+"/files/"+
			strconvID(detail.Files[0].ID)+"/info")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("info for %s = %d, want 200", movie.Title, res.StatusCode)
		}
		info := decodeStreamInfo(t, res)
		wantMode, wantReason := "direct", "container_direct"
		if strings.HasSuffix(detail.Files[0].Path, ".mkv") {
			wantMode, wantReason = "remux", "container_remux"
		}
		if info.Mode != wantMode || info.ReasonCode != wantReason {
			t.Errorf("%s decision = %q/%q, want %q/%q",
				detail.Files[0].Path, info.Mode, info.ReasonCode, wantMode, wantReason)
		}
		if info.FileID != detail.Files[0].ID || info.MovieID != movie.ID {
			t.Errorf("%s identity = film %d file %d, want %d/%d",
				movie.Title, info.MovieID, info.FileID, movie.ID, detail.Files[0].ID)
		}
		if info.FFmpegReady {
			t.Error("ffmpeg_ready = true with no binary on disk")
		}
	}
}

// TestStreamInfoMeasuredContractsCoverDirectAndUnsupported pins the two full
// decisions once a file has been measured, on a machine that has never
// downloaded ffmpeg. The unsupported answer carries the audio action with it
// (decision 1's bug, fixed in the stream package) and offers no quality rung
// at all when no encoder exists -- decision 59's honesty rule: the interface
// never offers a button this machine cannot press.
func TestStreamInfoMeasuredContractsCoverDirectAndUnsupported(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Alpha (2010).mp4", "an ordinary h264 film")
	testMediaFile(t, root, "Bravo (1993).mp4", "an mpeg2 film no browser decodes")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, _ := service.List(t.Context(), defaultProfileID, 10, 0)

	for _, movie := range movies {
		detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
		direct := strings.Contains(detail.Files[0].Path, "Alpha")
		media := library.FileMedia{
			Status:          library.MediaOK,
			Container:       "mov,mp4,m4a,3gp,3g2,mj2",
			DurationSeconds: 100,
			AudioTracks: []library.AudioTrack{
				{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true},
			},
		}
		if direct {
			media.Video = &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1280, Height: 720}
		} else {
			media.AudioTracks[0].Codec = "mp2"
			media.Video = &library.VideoStream{StreamIndex: 0, Codec: "mpeg2video", Width: 720, Height: 576}
		}
		if _, err := service.SaveFileMedia(t.Context(), movie.ID, detail.Files[0].ID, media); err != nil {
			t.Fatal(err)
		}

		res := get(t, handler, "/api/stream/"+strconvID(movie.ID)+"/files/"+
			strconvID(detail.Files[0].ID)+"/info")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("measured info = %d, want 200", res.StatusCode)
		}
		info := decodeStreamInfo(t, res)
		if direct {
			if info.Mode != "direct" || info.ReasonCode != "direct_play" {
				t.Errorf("h264 decision = %q/%q, want direct/direct_play", info.Mode, info.ReasonCode)
			}
			if info.VideoCodec != "h264" {
				t.Errorf("video_codec = %q, want the lowercase measured codec", info.VideoCodec)
			}
			continue
		}
		if info.Mode != "unsupported" || info.ReasonCode != "video_transcode_required" {
			t.Errorf("mpeg2 decision = %q/%q, want unsupported/video_transcode_required",
				info.Mode, info.ReasonCode)
		}
		if info.Transcode == nil || info.Transcode.Available {
			t.Error("transcode claims availability with no encoder measured")
		}
		if len(info.Qualities) != 1 || info.Qualities[0].Height != 0 {
			t.Errorf("qualities = %+v, want the single source rung", info.Qualities)
		}
	}
}

// TestSeekStartValidatesPositionAndRefusesWithoutFFmpeg pins the seek clock's
// front door. The position is validated before anything else (400 for a
// request that is not a time), and a machine without ffmpeg answers
// seek_start_unavailable rather than downloading (the M1 promise). The
// keyframe echo and its backwards-only clamp need a real binary and live in
// the playback suite for now.
func TestSeekStartValidatesPositionAndRefusesWithoutFFmpeg(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Heat (1995).mp4", "ordinary bytes")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
	seek := "/api/stream/" + strconvID(movie.ID) + "/files/" +
		strconvID(detail.Files[0].ID) + "/seek"

	for _, bad := range []string{"?t=abc", "?t=-5", "?t=", ""} {
		if res := get(t, handler, seek+bad); res.StatusCode != http.StatusBadRequest ||
			apiErrorCode(t, res) != "invalid_position" {
			t.Errorf("seek with %q = %d/%s, want 400 invalid_position", bad, res.StatusCode, apiErrorCode(t, res))
		}
	}
	if res := get(t, handler, seek+"?t=100"); res.StatusCode != http.StatusNotFound ||
		apiErrorCode(t, res) != "seek_start_unavailable" {
		t.Errorf("seek without ffmpeg = %d/%s, want 404 seek_start_unavailable",
			res.StatusCode, apiErrorCode(t, res))
	}
}

// TestLegacyStreamRoutesServeTheSameContract pins the fourth wiring of the
// playback feature. Legacy /info delegates to the per-file handler through the
// primary file, so its body must match; legacy remux hits the same guards; and
// legacy direct play serves bytes with ranges. One documented gap stays as
// found: legacy direct play does not refuse ?audio=, unlike the per-file
// route -- decision 59 territory, unchanged until a contract change is
// validated separately.
func TestLegacyStreamRoutesServeTheSameContract(t *testing.T) {
	handler, service, root := newPlaybackGuardServer(t)
	contents := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 20)
	testMediaFile(t, root, "Heat (1995).mp4", contents)
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
	perFile := "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID)

	legacyInfo := get(t, handler, "/api/stream/"+strconvID(movie.ID)+"/info")
	fileInfo := get(t, handler, perFile+"/info")
	if legacyInfo.StatusCode != http.StatusOK || fileInfo.StatusCode != http.StatusOK {
		t.Fatalf("info statuses = legacy %d file %d, want 200/200",
			legacyInfo.StatusCode, fileInfo.StatusCode)
	}
	legacy := decodeStreamInfo(t, legacyInfo)
	file := decodeStreamInfo(t, fileInfo)
	if legacy.Mode != file.Mode || legacy.ReasonCode != file.ReasonCode ||
		legacy.Container != file.Container || legacy.FileID != file.FileID ||
		legacy.MovieID != file.MovieID {
		t.Errorf("legacy info %+v differs from per-file info %+v", legacy, file)
	}

	if res := get(t, handler, "/api/stream/"+strconvID(movie.ID)+"/remux"); res.StatusCode != http.StatusServiceUnavailable ||
		apiErrorCode(t, res) != "ffmpeg_unavailable" {
		t.Errorf("legacy remux guard = %d/%s, want 503 ffmpeg_unavailable", res.StatusCode, apiErrorCode(t, res))
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/stream/"+strconvID(movie.ID), nil)
	request.Header.Set("Range", "bytes=0-4")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusPartialContent || recorder.Body.String() != contents[:5] {
		t.Errorf("legacy range = %d %q, want 206 %q", recorder.Code, recorder.Body.String(), contents[:5])
	}

	// The gap, recorded on purpose: the per-file direct route refuses an audio
	// selection, the legacy one silently ignores it.
	if res := get(t, handler, "/api/stream/"+strconvID(movie.ID)+"?audio=1"); res.StatusCode != http.StatusOK {
		t.Errorf("legacy direct with ?audio=1 = %d, want 200 (the documented gap)", res.StatusCode)
	}
}

// TestRemuxGuardsOnBothTwins pins the first gate of each remux route and the
// one ordering asymmetry between them: the film handler validates ?h= before
// anything else, the episode handler does not, so a bogus height on an episode
// reaches the ffmpeg guard instead. Tranche 2 must preserve this difference
// byte for byte, or supersede it consciously -- it is the kind of drift the
// unified planner is meant to remove, and the golden tests hold both truths
// until then.
func TestRemuxGuardsOnBothTwins(t *testing.T) {
	handler, service, root := newPlaybackGuardServer(t)
	testMediaFile(t, root, "Heat (1995).mkv", "an mkv for remuxing")
	writeEpisodeMedia(t, root, "Shows/Severance (2022)/Season 01/S01E01.720p.mkv", "an episode mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
	filmRemux := "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID) + "/remux"

	seriesList, err := service.ListSeries(t.Context(), 10, 0)
	if err != nil || len(seriesList) != 1 {
		t.Fatalf("series = %d, err = %v, want one", len(seriesList), err)
	}
	season, err := service.GetSeason(t.Context(), defaultProfileID, seriesList[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	episode, err := service.GetEpisodeItem(t.Context(), defaultProfileID, season.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	episodeRemux := "/api/library/episodes/" + strconvID(episode.ID) + "/files/" +
		strconvID(episode.Files[0].ID) + "/stream/remux"

	if res := get(t, handler, filmRemux+"?h=999"); res.StatusCode != http.StatusBadRequest ||
		apiErrorCode(t, res) != "invalid_height" {
		t.Errorf("film remux with a bogus height = %d/%s, want 400 invalid_height",
			res.StatusCode, apiErrorCode(t, res))
	}
	if res := get(t, handler, episodeRemux+"?h=999"); res.StatusCode != http.StatusServiceUnavailable ||
		apiErrorCode(t, res) != "ffmpeg_unavailable" {
		t.Errorf("episode remux with a bogus height = %d/%s, want 503 ffmpeg_unavailable (the pinned asymmetry)",
			res.StatusCode, apiErrorCode(t, res))
	}
	if res := get(t, handler, filmRemux); res.StatusCode != http.StatusServiceUnavailable ||
		apiErrorCode(t, res) != "ffmpeg_unavailable" {
		t.Errorf("film remux without ffmpeg = %d/%s, want 503", res.StatusCode, apiErrorCode(t, res))
	}
	if res := get(t, handler, episodeRemux); res.StatusCode != http.StatusServiceUnavailable ||
		apiErrorCode(t, res) != "ffmpeg_unavailable" {
		t.Errorf("episode remux without ffmpeg = %d/%s, want 503", res.StatusCode, apiErrorCode(t, res))
	}
}

// TestExternalSidecarIsOfferedAndRebasedToTheSeekClock pins the sidecar path
// end to end without ffmpeg: /info notices the .srt beside the film, the track
// is served as WebVTT, and the same ?t= that seeks the picture rebases the
// text. Cues that end before the seek are dropped entirely -- a subtitle
// sitting in the past is worse than a missing one.
func TestExternalSidecarIsOfferedAndRebasedToTheSeekClock(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Contact (1997).mp4", "a film that plays directly")
	testMediaFile(t, root, "Contact (1997).srt", "1\n00:00:10,000 --> 00:00:14,000\nearly\n\n"+
		"2\n00:01:10,000 --> 00:01:14,000\nlate\n")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)

	info := decodeStreamInfo(t, get(t, handler, "/api/stream/"+strconvID(movie.ID)+"/files/"+
		strconvID(detail.Files[0].ID)+"/info"))
	if len(info.SubtitleTracks) != 1 || !info.SubtitleTracks[0].IsExternal {
		t.Fatalf("subtitle tracks = %+v, want one external sidecar", info.SubtitleTracks)
	}
	track := info.SubtitleTracks[0]

	url := "/api/library/movies/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID) +
		"/subtitles/" + strconvID(track.ID) + "?t=60"
	res := get(t, handler, url)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("subtitle response = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/vtt") {
		t.Errorf("Content-Type = %q, want text/vtt", ct)
	}
	body, _ := io.ReadAll(res.Body)
	text := string(body)
	if !strings.HasPrefix(text, "WEBVTT") {
		t.Errorf("subtitle body = %q, want a WEBVTT document", text)
	}
	if !strings.Contains(text, "00:00:10.000") || !strings.Contains(text, "late") {
		t.Errorf("the 70s cue was not rebased to 10s: %q", text)
	}
	if strings.Contains(text, "early") {
		t.Errorf("a cue ending at 14s survived a 60s seek: %q", text)
	}
}

// TestBitmapSubtitleTrackIsNamedAndRefused pins decision 3: an image track is
// not hidden, it is refused with its own code so the interface can say which
// one and why.
func TestBitmapSubtitleTrackIsNamedAndRefused(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Blade Runner (1982).mkv", "a film with a bitmap track")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyAPIMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
	streamIndex := 3
	if _, err := service.SaveFileMedia(t.Context(), movie.ID, detail.Files[0].ID, library.FileMedia{
		Status:          library.MediaOK,
		Container:       "matroska,webm",
		DurationSeconds: 117,
		Video:           &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks:     []library.AudioTrack{{StreamIndex: 1, Codec: "dts", Language: "eng", IsDefault: true}},
		SubtitleTracks: []library.SubtitleTrack{
			{StreamIndex: &streamIndex, Codec: "hdmv_pgs_subtitle", Language: "fre"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	info := decodeStreamInfo(t, get(t, handler, "/api/stream/"+strconvID(movie.ID)+"/files/"+
		strconvID(detail.Files[0].ID)+"/info"))
	if len(info.SubtitleTracks) != 1 || info.SubtitleTracks[0].Kind != "image" {
		t.Fatalf("subtitle tracks = %+v, want one image-kind track", info.SubtitleTracks)
	}
	track := info.SubtitleTracks[0]

	res := get(t, handler, "/api/library/movies/"+strconvID(movie.ID)+"/files/"+
		strconvID(detail.Files[0].ID)+"/subtitles/"+strconvID(track.ID))
	if res.StatusCode != http.StatusUnsupportedMediaType || apiErrorCode(t, res) != "subtitle_image_based" {
		t.Errorf("bitmap track = %d/%s, want 415 subtitle_image_based", res.StatusCode, apiErrorCode(t, res))
	}
}

// TestEpisodeInfoUnmeasuredMatchesTheFilmContainerDecision pins the episode
// twin's container-only answer next to the film one, so tranche 2 can prove
// the unified planner did not change either side.
func TestEpisodeInfoUnmeasuredMatchesTheFilmContainerDecision(t *testing.T) {
	handler, service, _, items := episodeFixture(t)
	item, err := service.GetEpisodeItem(t.Context(), defaultProfileID, items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	res := get(t, handler, "/api/library/episodes/"+strconvID(item.ID)+"/files/"+
		strconvID(item.Files[0].ID)+"/stream/info")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("episode info = %d, want 200", res.StatusCode)
	}
	info := decodeStreamInfo(t, res)
	if info.Mode != "direct" || info.ReasonCode != "container_direct" || info.EpisodeID != item.ID {
		t.Errorf("episode decision = %+v, want direct/container_direct on the episode identity", info)
	}
}
