package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLegacyInfoUsesPrimaryFileAndWatchlistRoutes(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Arrival.2016.mp4", "fixture")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), 1, 10, 0)
	if err != nil || len(movies) != 1 {
		t.Fatalf("movies=%v err=%v", movies, err)
	}
	id := strconvID(movies[0].ID)
	response := get(t, handler, "/api/stream/"+id+"/info")
	defer response.Body.Close()
	var info streamInfoResponse
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || info.FileID == 0 || info.Mode != "direct" {
		t.Fatalf("legacy info = %+v status=%d", info, response.StatusCode)
	}
	for _, method := range []string{http.MethodPut, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/library/movies/"+id+"/watchlist", nil))
		if rec.Code != 204 {
			t.Fatalf("%s watchlist = %d %s", method, rec.Code, rec.Body.String())
		}
	}
	rec := get(t, handler, "/api/library/watchlist?profile=999")
	defer rec.Body.Close()
	if rec.StatusCode != 404 {
		t.Fatalf("missing profile = %d", rec.StatusCode)
	}
}
