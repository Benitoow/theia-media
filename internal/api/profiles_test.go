package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func decodeInto(t *testing.T, res *http.Response, target any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
}

func do(t *testing.T, h http.Handler, method, path, body string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	var request *http.Request
	if body == "" {
		request = httptest.NewRequest(method, path, nil)
	} else {
		request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
	}
	h.ServeHTTP(rec, request)
	return rec.Result()
}

// The whole point of M2: two viewers, one film, two positions that do not touch.
func TestProgressIsSeparatePerProfileAndTheDefaultStillWorks(t *testing.T) {
	handler, service, root := newMovieFileTestServer(t)
	testMediaFile(t, root, "Solaris 1972.mkv", "0123456789")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil || len(movies) != 1 {
		t.Fatalf("movies = %v, err = %v", movies, err)
	}
	film := strconv.FormatInt(movies[0].ID, 10)

	created := do(t, handler, http.MethodPost, "/api/profiles", `{"name":"Mimi"}`)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create profile = %d", created.StatusCode)
	}
	var second struct {
		ID int64 `json:"id"`
	}
	decodeInto(t, created, &second)

	// The default profile is whoever a request without ?profile= speaks for,
	// which is what keeps the released frontend working.
	if res := do(t, handler, http.MethodPut, "/api/library/movies/"+film+"/progress",
		`{"position_seconds":600,"duration_seconds":3600}`); res.StatusCode != http.StatusOK {
		t.Fatalf("default progress = %d", res.StatusCode)
	}
	if res := do(t, handler, http.MethodPut,
		"/api/library/movies/"+film+"/progress?profile="+strconv.FormatInt(second.ID, 10),
		`{"position_seconds":1800,"duration_seconds":3600}`); res.StatusCode != http.StatusOK {
		t.Fatalf("second profile progress = %d", res.StatusCode)
	}

	for _, want := range []struct {
		query    string
		position float64
	}{
		{"", 600},
		{"?profile=" + strconv.FormatInt(second.ID, 10), 1800},
	} {
		res := do(t, handler, http.MethodGet, "/api/library/movies/"+film+want.query, "")
		var movie struct {
			Progress struct {
				PositionSeconds float64 `json:"position_seconds"`
			} `json:"progress"`
		}
		decodeInto(t, res, &movie)
		if movie.Progress.PositionSeconds != want.position {
			t.Errorf("position for %q = %v, want %v",
				want.query, movie.Progress.PositionSeconds, want.position)
		}
	}

	// Resetting one viewer must not disturb the other.
	if res := do(t, handler, http.MethodDelete,
		"/api/library/movies/"+film+"/progress?profile="+strconv.FormatInt(second.ID, 10),
		""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("reset = %d", res.StatusCode)
	}
	res := do(t, handler, http.MethodGet, "/api/library/movies/"+film, "")
	var afterReset struct {
		Progress struct {
			PositionSeconds float64 `json:"position_seconds"`
		} `json:"progress"`
	}
	decodeInto(t, res, &afterReset)
	if afterReset.Progress.PositionSeconds != 600 {
		t.Errorf("default position after the other viewer reset = %v, want 600",
			afterReset.Progress.PositionSeconds)
	}
}

// The legacy columns are the rollback contract with v1.5.0, and only the
// default profile may write them.
func TestOnlyTheDefaultProfileMirrorsTheLegacyColumns(t *testing.T) {
	handler, service, root, database := newMovieFileTestServerDB(t)
	testMediaFile(t, root, "Network 1976.mkv", "0123456789")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, _ := service.List(t.Context(), defaultProfileID, 10, 0)
	film := strconv.FormatInt(movies[0].ID, 10)

	created := do(t, handler, http.MethodPost, "/api/profiles", `{"name":"Diegoat"}`)
	var second struct {
		ID int64 `json:"id"`
	}
	decodeInto(t, created, &second)

	do(t, handler, http.MethodPut, "/api/library/movies/"+film+"/progress?profile="+
		strconv.FormatInt(second.ID, 10), `{"position_seconds":900,"duration_seconds":3600}`)

	// v1.5.0 reads the film row directly, so it must still see zero here.
	var legacy float64
	if err := database.QueryRowContext(t.Context(),
		`SELECT position_seconds FROM movies WHERE id = ?`, movies[0].ID).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if legacy != 0 {
		t.Errorf("legacy column = %v after a non-default viewer watched, want 0", legacy)
	}

	do(t, handler, http.MethodPut, "/api/library/movies/"+film+"/progress",
		`{"position_seconds":120,"duration_seconds":3600}`)
	if err := database.QueryRowContext(t.Context(),
		`SELECT position_seconds FROM movies WHERE id = ?`, movies[0].ID).Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if legacy != 120 {
		t.Errorf("legacy column = %v after the default viewer watched, want 120", legacy)
	}
}

func TestProfileRoutesRefuseWhatTheyShould(t *testing.T) {
	handler, _, _ := newMovieFileTestServer(t)

	cases := []struct {
		method, path, body string
		status             int
		code               string
	}{
		{http.MethodGet, "/api/library/movies?profile=abc", "", http.StatusBadRequest, "invalid_profile_id"},
		{http.MethodGet, "/api/library/movies?profile=999", "", http.StatusNotFound, "profile_not_found"},
		{http.MethodPost, "/api/profiles", `{"name":"   "}`, http.StatusBadRequest, "invalid_profile_name"},
		{http.MethodPost, "/api/profiles", `{"nickname":"x"}`, http.StatusBadRequest, "invalid_profile_payload"},
		{http.MethodGet, "/api/profiles/999", "", http.StatusNotFound, "profile_not_found"},
		{http.MethodDelete, "/api/profiles/1", "", http.StatusConflict, "profile_last_remaining"},
		{http.MethodGet, "/api/profiles/1/avatar", "", http.StatusNotFound, "profile_image_not_found"},
		{http.MethodPut, "/api/profiles/1/avatar", "not an image", http.StatusUnsupportedMediaType, "profile_image_unreadable"},
	}
	for _, tc := range cases {
		res := do(t, handler, tc.method, tc.path, tc.body)
		if res.StatusCode != tc.status {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, res.StatusCode, tc.status)
			continue
		}
		var body struct {
			Error string `json:"error"`
		}
		decodeInto(t, res, &body)
		if body.Error != tc.code {
			t.Errorf("%s %s error = %q, want %q", tc.method, tc.path, body.Error, tc.code)
		}
	}
}
