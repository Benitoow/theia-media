package api

// Golden bodies for /info, recorded from the code as tranche 2 found it. The
// refactor moved the delivery decision into internal/playback; these files
// are the byte-for-byte proof that the answers did not move with it.
// Regenerate with `go test ./internal/api/ -run TestPlaybackInfo -update` --
// and read the diff before committing it.
//
// Identifiers are deterministic on a fresh database (scan order and
// autoincrement), so the bodies are stable across runs and machines.

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Benitoow/theia-media/internal/library"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden playback bodies")

func goldenBody(t *testing.T, name string, body []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the golden body: %v", err)
	}
	if !bytes.Equal(want, body) {
		t.Errorf("%s drifted from the recorded contract:\nwant %q\ngot  %q", name, want, body)
	}
}

func TestPlaybackInfoBodiesAreByteStable(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) (http.Handler, string)
	}{
		{
			name: "film_mp4_measured_direct",
			setup: func(t *testing.T) (http.Handler, string) {
				handler, service, root := newMovieFileTestServer(t)
				testMediaFile(t, root, "Alpha (2010).mp4", "friendly bytes")
				scanOne(t, service, root)
				movie := onlyAPIMovie(t, service)
				detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
				_, err := service.SaveFileMedia(t.Context(), movie.ID, detail.Files[0].ID, library.FileMedia{
					Status:          library.MediaOK,
					Container:       "mov,mp4,m4a,3gp,3g2,mj2",
					DurationSeconds: 115,
					Video:           &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080, FrameRate: 23.976},
					AudioTracks:     []library.AudioTrack{{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true}},
				})
				if err != nil {
					t.Fatal(err)
				}
				return handler, "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID) + "/info"
			},
		},
		{
			name: "film_mkv_unmeasured_remux",
			setup: func(t *testing.T) (http.Handler, string) {
				handler, service, root := newMovieFileTestServer(t)
				testMediaFile(t, root, "Heat (1995).mkv", "needs rewrapping")
				scanOne(t, service, root)
				movie := onlyAPIMovie(t, service)
				detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
				return handler, "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID) + "/info"
			},
		},
		{
			name: "film_mpeg2_unsupported",
			setup: func(t *testing.T) (http.Handler, string) {
				handler, service, root := newMovieFileTestServer(t)
				testMediaFile(t, root, "Bravo (1993).mp4", "mpeg2 bytes")
				scanOne(t, service, root)
				movie := onlyAPIMovie(t, service)
				detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
				_, err := service.SaveFileMedia(t.Context(), movie.ID, detail.Files[0].ID, library.FileMedia{
					Status:          library.MediaOK,
					Container:       "mov,mp4,m4a,3gp,3g2,mj2",
					DurationSeconds: 126,
					Video:           &library.VideoStream{StreamIndex: 0, Codec: "mpeg2video", Width: 720, Height: 576},
					AudioTracks:     []library.AudioTrack{{StreamIndex: 1, Codec: "mp2", Language: "rus", IsDefault: true}},
				})
				if err != nil {
					t.Fatal(err)
				}
				return handler, "/api/stream/" + strconvID(movie.ID) + "/files/" + strconvID(detail.Files[0].ID) + "/info"
			},
		},
		{
			name: "film_selected_audio_remux_tone_map",
			setup: func(t *testing.T) (http.Handler, string) {
				handler, service, root := newMovieFileTestServer(t)
				testMediaFile(t, root, "Heat (1995).mp4", "pq bytes")
				scanOne(t, service, root)
				movie := onlyAPIMovie(t, service)
				detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
				file, err := service.SaveFileMedia(t.Context(), movie.ID, detail.Files[0].ID, library.FileMedia{
					Status:          library.MediaOK,
					Container:       "mov,mp4,m4a,3gp,3g2,mj2",
					DurationSeconds: 100,
					Video: &library.VideoStream{StreamIndex: 0, Codec: "h264", Width: 3840, Height: 2160,
						ColorTransfer: "smpte2084"},
					AudioTracks: []library.AudioTrack{
						{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true},
						{StreamIndex: 2, Codec: "aac", Language: "fre"},
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				trackID := file.Media.AudioTracks[1].ID
				return handler, "/api/stream/" + strconvID(movie.ID) + "/files/" +
					strconvID(file.ID) + "/info?audio=" + strconvID(trackID)
			},
		},
		{
			name: "episode_mkv_unmeasured_remux",
			setup: func(t *testing.T) (http.Handler, string) {
				handler, service, root := newMovieFileTestServer(t)
				writeEpisodeMedia(t, root, "Shows/Severance (2022)/Season 01/S01E01.720p.mkv", "episode bytes")
				if _, err := service.Scan(t.Context(), []string{root}); err != nil {
					t.Fatal(err)
				}
				seriesList, err := service.ListSeries(t.Context(), 10, 0)
				if err != nil || len(seriesList) != 1 {
					t.Fatalf("series = %d, err = %v", len(seriesList), err)
				}
				season, err := service.GetSeason(t.Context(), defaultProfileID, seriesList[0].ID, 1)
				if err != nil {
					t.Fatal(err)
				}
				item, err := service.GetEpisodeItem(t.Context(), defaultProfileID, season.Items[0].ID)
				if err != nil {
					t.Fatal(err)
				}
				return handler, "/api/library/episodes/" + strconvID(item.ID) + "/files/" +
					strconvID(item.Files[0].ID) + "/stream/info"
			},
		},
		{
			name: "legacy_film_info_primary_file",
			setup: func(t *testing.T) (http.Handler, string) {
				handler, service, root := newMovieFileTestServer(t)
				testMediaFile(t, root, "Heat (1995).mp4", "legacy bytes")
				scanOne(t, service, root)
				movie := onlyAPIMovie(t, service)
				return handler, "/api/stream/" + strconvID(movie.ID) + "/info"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, url := tc.setup(t)
			res := get(t, handler, url)
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", res.StatusCode)
			}
			goldenBody(t, tc.name, readAllBody(t, res))
		})
	}
}

func scanOne(t *testing.T, service *library.Service, root string) {
	t.Helper()
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
}

func readAllBody(t *testing.T, res *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	return body
}
