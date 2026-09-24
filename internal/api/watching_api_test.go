package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Benitoow/theia-media/internal/library"
)

// TestWatchStatsAnswersForTheAskingProfile covers what the settings screen
// depends on: the route exists, the numbers belong to the profile that asked,
// and a profile that does not exist is refused rather than answered with the
// default viewer's evening.
func TestWatchStatsAnswersForTheAskingProfile(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Ran 1985 1080p.mkv", "a film")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil || len(movies) != 1 {
		t.Fatalf("movies=%v err=%v", movies, err)
	}
	// 7100 of 7200 leaves 100 seconds, inside the two-minute window, so this
	// film counts as watched and not merely as started.
	if _, err := service.SaveProgress(t.Context(), defaultProfileID,
		movies[0].ID, 7100, 7200); err != nil {
		t.Fatal(err)
	}

	res := get(t, handler, "/api/library/watching")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("watching = %d, want 200", res.StatusCode)
	}
	var stats library.WatchStats
	if err := json.NewDecoder(res.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.Movies.Started != 1 || stats.Movies.Finished != 1 || stats.Movies.Seconds != 7100 {
		t.Errorf("movies = %+v, want one film watched and finished", stats.Movies)
	}
	if stats.Top == nil {
		t.Error("top_series came back null, want an empty list a client can iterate")
	}

	for query, want := range map[string]int{
		"?profile=abc": http.StatusBadRequest,
		"?profile=999": http.StatusNotFound,
	} {
		if res := get(t, handler, "/api/library/watching"+query); res.StatusCode != want {
			t.Errorf("watching%s = %d, want %d", query, res.StatusCode, want)
		}
	}
}
