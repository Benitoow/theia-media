package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient wires a client to a stub API with rate limiting disabled, so
// the tests do not pay 50ms per request.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New("test-token", WithBaseURL(server.URL), WithImageBaseURL(server.URL),
		WithRequestInterval(0))
}

func TestSearchSendsTheYearAndABearerToken(t *testing.T) {
	var got url.Values
	var auth string

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"results":[{"id":603,"title":"The Matrix"}]}`))
	})

	id, err := client.Search(t.Context(), "The Matrix", 1999)
	if err != nil {
		t.Fatalf("Search returned an unexpected error: %v", err)
	}
	if id != 603 {
		t.Errorf("id = %d, want 603", id)
	}
	if got.Get("query") != "The Matrix" {
		t.Errorf("query = %q", got.Get("query"))
	}
	if got.Get("primary_release_year") != "1999" {
		t.Errorf("primary_release_year = %q, want 1999", got.Get("primary_release_year"))
	}
	// A key in the query string ends up in every proxy log between here and
	// TMDB; it belongs in a header.
	if auth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want a bearer token", auth)
	}
	if strings.Contains(got.Encode(), "test-token") {
		t.Error("the token leaked into the query string")
	}
}

func TestSearchPrefersAnExactTitleMatch(t *testing.T) {
	// TMDB orders by popularity. A popular near-miss at the top should lose to
	// an exact match further down.
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[
			{"id":1,"title":"The Matrix Reloaded"},
			{"id":2,"title":"The Matrix"}
		]}`))
	})

	id, err := client.Search(t.Context(), "The Matrix", 0)
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Errorf("id = %d, want the exact match 2", id)
	}
}

func TestSearchRetriesWithoutTheYear(t *testing.T) {
	// A filename can carry the wrong year. A slightly wrong match beats none.
	var calls atomic.Int32

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			if r.URL.Query().Get("primary_release_year") == "" {
				t.Error("the first attempt should carry the year")
			}
			w.Write([]byte(`{"results":[]}`))
			return
		}
		if r.URL.Query().Get("primary_release_year") != "" {
			t.Error("the retry should drop the year")
		}
		w.Write([]byte(`{"results":[{"id":42,"title":"Solaris"}]}`))
	})

	id, err := client.Search(t.Context(), "Solaris", 1971)
	if err != nil {
		t.Fatalf("Search returned an unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if calls.Load() != 2 {
		t.Errorf("made %d calls, want 2", calls.Load())
	}
}

func TestEmptyResultsAreNotFoundRatherThanAnError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[]}`))
	})

	_, err := client.Search(t.Context(), "Nothing At All", 0)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestBlankTitleNeverReachesTheNetwork(t *testing.T) {
	var called atomic.Bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.Write([]byte(`{"results":[]}`))
	})

	if _, err := client.Search(t.Context(), "   ", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if called.Load() {
		t.Error("a blank title produced an API call")
	}
}

func TestUnauthorizedIsDistinguishedAndNotRetried(t *testing.T) {
	// Every later lookup would fail identically, so this has to be reported
	// rather than retried into the ground.
	var calls atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := client.Search(t.Context(), "Anything", 0)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
	if calls.Load() != 1 {
		t.Errorf("made %d calls, want exactly 1", calls.Load())
	}
}

func TestServerErrorsAreRetried(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(`{"results":[{"id":7,"title":"Stalker"}]}`))
	})

	// The backoff is a second per attempt, so give this room.
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	id, err := client.Search(ctx, "Stalker", 0)
	if err != nil {
		t.Fatalf("Search returned an unexpected error: %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}
	if calls.Load() != 3 {
		t.Errorf("made %d calls, want 3", calls.Load())
	}
}

func TestDetailsExtractsCreditsAndDirector(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "append_to_response=credits") {
			t.Error("credits should be requested in the same round trip")
		}
		w.Write([]byte(`{
			"id": 603,
			"title": "Matrix",
			"overview": "Un pirate informatique…",
			"release_date": "1999-03-31",
			"poster_path": "/poster.jpg",
			"backdrop_path": "/backdrop.jpg",
			"runtime": 136,
			"vote_average": 8.2,
			"genres": [{"name":"Action"},{"name":"Science-Fiction"}],
			"credits": {
				"cast": [{"name":"Keanu Reeves","character":"Neo"}],
				"crew": [
					{"name":"Somebody Else","job":"Producer"},
					{"name":"Lana Wachowski","job":"Director"}
				]
			}
		}`))
	})

	film, err := client.Details(t.Context(), 603)
	if err != nil {
		t.Fatalf("Details returned an unexpected error: %v", err)
	}
	if film.Title != "Matrix" || film.Runtime != 136 {
		t.Errorf("film = %+v", film)
	}
	if film.Director != "Lana Wachowski" {
		t.Errorf("director = %q, want Lana Wachowski", film.Director)
	}
	if len(film.Genres) != 2 || film.Genres[0] != "Action" {
		t.Errorf("genres = %v", film.Genres)
	}
	if len(film.Cast) != 1 || film.Cast[0].Character != "Neo" {
		t.Errorf("cast = %+v", film.Cast)
	}
}

func TestDetailsCapsTheCastList(t *testing.T) {
	var big strings.Builder
	big.WriteString(`{"id":1,"credits":{"cast":[`)
	for i := range 40 {
		if i > 0 {
			big.WriteString(",")
		}
		big.WriteString(`{"name":"Someone","character":"Role"}`)
	}
	big.WriteString(`]}}`)

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(big.String()))
	})

	film, err := client.Details(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(film.Cast) != maxCast {
		t.Errorf("cast has %d entries, want it capped at %d", len(film.Cast), maxCast)
	}
}

func TestImageRequestsCarryNoCredentials(t *testing.T) {
	// The image CDN is public and has no business seeing the bearer token.
	var auth string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("not really a jpeg"))
	})

	data, contentType, err := client.FetchImage(t.Context(), "w342", "/poster.jpg")
	if err != nil {
		t.Fatalf("FetchImage returned an unexpected error: %v", err)
	}
	if auth != "" {
		t.Errorf("the image request carried Authorization: %q", auth)
	}
	if contentType != "image/jpeg" || len(data) == 0 {
		t.Errorf("contentType = %q, %d bytes", contentType, len(data))
	}
}

func TestNormalise(t *testing.T) {
	tests := []struct{ in, want string }{
		{"The Matrix", "the matrix"},
		{"The Matrix.", "the matrix"},
		{"  Spider-Man:  Far   From Home ", "spider man far from home"},
		{"WALL·E", "wall e"},
		// Accents carry meaning in French and must survive.
		{"Amélie", "amélie"},
	}
	for _, tt := range tests {
		if got := normalise(tt.in); got != tt.want {
			t.Errorf("normalise(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLimiterSpacesRequestsApart(t *testing.T) {
	l := &limiter{interval: 20 * time.Millisecond}

	start := time.Now()
	for range 4 {
		if err := l.wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	// Four slots at 20ms: the first is free, the rest wait. Allow slack for a
	// loaded CI machine but require that spacing happened at all.
	if elapsed := time.Since(start); elapsed < 55*time.Millisecond {
		t.Errorf("four requests took %v, want at least ~60ms of spacing", elapsed)
	}
}
