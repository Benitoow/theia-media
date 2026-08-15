package library

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/tmdb"
)

// stubTMDB stands in for the real service: a search that answers with the wrong
// film, and two records to fetch by id. It records every path it was asked for,
// which is what the pinning tests actually assert on.
type stubTMDB struct {
	mu    sync.Mutex
	paths []string
}

func (s *stubTMDB) record(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paths = append(s.paths, path)
}

func (s *stubTMDB) asked(fragment string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, p := range s.paths {
		if strings.Contains(p, fragment) {
			n++
		}
	}
	return n
}

// newMatchTestService is newTestService with a TMDB client pointed at the stub.
func newMatchTestService(t *testing.T) (*Service, string, *stubTMDB) {
	t.Helper()

	stub := &stubTMDB{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.record(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/search/movie":
			// "Making of The Handmaiden" all over again: the search leads
			// somewhere plausible and wrong.
			_, _ = w.Write([]byte(`{"results":[
				{"id":1,"title":"Le mauvais film","original_title":"The Wrong One",
				 "release_date":"1999-01-01","popularity":9,"overview":"non",
				 "poster_path":"/wrong.jpg"},
				{"id":2,"title":"Le bon film","original_title":"The Right One",
				 "release_date":"1999-06-01","popularity":4,"overview":"oui",
				 "poster_path":"/right.jpg"}]}`))
		case strings.HasPrefix(r.URL.Path, "/movie/1"):
			_, _ = w.Write([]byte(`{"id":1,"title":"Le mauvais film","overview":"non",
				"release_date":"1999-01-01","poster_path":"/wrong.jpg","runtime":90,
				"vote_average":4.0,"genres":[],"credits":{"cast":[],"crew":[]}}`))
		case strings.HasPrefix(r.URL.Path, "/movie/2"):
			_, _ = w.Write([]byte(`{"id":2,"title":"Le bon film","overview":"oui",
				"release_date":"1999-06-01","poster_path":"/right.jpg","runtime":120,
				"vote_average":8.0,"genres":[],"credits":{"cast":[],"crew":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	client := tmdb.New("test-token", tmdb.WithBaseURL(server.URL),
		tmdb.WithImageBaseURL(server.URL), tmdb.WithRequestInterval(0))

	service := NewService(NewStore(database), client, slog.New(slog.DiscardHandler))
	return service, t.TempDir(), stub
}

// scanned returns the one film in the library after a scan.
func scannedFilm(t *testing.T, service *Service) Movie {
	t.Helper()
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 {
		t.Fatalf("the library holds %d films, want 1", len(movies))
	}
	return movies[0]
}

func TestCorrectingAMatchReplacesTheMetadata(t *testing.T) {
	service, root, _ := newMatchTestService(t)
	writeFile(t, root, "Le film (1999).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	film := scannedFilm(t, service)
	if film.Metadata.TMDBID != 1 {
		t.Fatalf("the automatic match chose %d, want the wrong film 1", film.Metadata.TMDBID)
	}

	corrected, err := service.SetMovieMatch(t.Context(), defaultProfileID, film.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if corrected.Metadata.TMDBID != 2 {
		t.Errorf("tmdb id = %d after the correction, want 2", corrected.Metadata.TMDBID)
	}
	if corrected.Metadata.Title != "Le bon film" {
		t.Errorf("title = %q after the correction, want %q", corrected.Metadata.Title, "Le bon film")
	}
	if corrected.Metadata.PosterPath != "/right.jpg" {
		t.Errorf("poster = %q, want the corrected one", corrected.Metadata.PosterPath)
	}
}

func TestACorrectedMatchSurvivesTheNextRefresh(t *testing.T) {
	// The whole point. Metadata is cached and never frozen (decision 9), so the
	// record is re-read eventually; if that re-read searched the filename again
	// it would find the same wrong film and quietly undo the correction.
	service, root, stub := newMatchTestService(t)
	writeFile(t, root, "Le film (1999).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	film := scannedFilm(t, service)
	if _, err := service.SetMovieMatch(t.Context(), defaultProfileID, film.ID, 2); err != nil {
		t.Fatal(err)
	}

	searchesBefore := stub.asked("/search/movie")

	// Age the record past its lifetime, which is what ninety days of ordinary
	// use does on its own.
	if _, err := service.store.db.ExecContext(t.Context(),
		`UPDATE movies SET metadata_fetched_at = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	if got := stub.asked("/search/movie"); got != searchesBefore {
		t.Errorf("the refresh searched the title again (%d searches, was %d)", got, searchesBefore)
	}
	if stub.asked("/movie/2") < 2 {
		t.Error("the refresh did not re-read the pinned record by id")
	}

	film = scannedFilm(t, service)
	if film.Metadata.TMDBID != 2 {
		t.Errorf("tmdb id = %d after a refresh, want the correction to have held at 2", film.Metadata.TMDBID)
	}
}

func TestClearingAMatchHandsTheFilmBackToTheMatcher(t *testing.T) {
	service, root, _ := newMatchTestService(t)
	writeFile(t, root, "Le film (1999).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	film := scannedFilm(t, service)
	if _, err := service.SetMovieMatch(t.Context(), defaultProfileID, film.ID, 2); err != nil {
		t.Fatal(err)
	}

	if err := service.ClearMovieMatch(t.Context(), film.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	film = scannedFilm(t, service)
	if film.Metadata.TMDBID != 1 {
		t.Errorf("tmdb id = %d, want the automatic match 1 back", film.Metadata.TMDBID)
	}
}

func TestCandidatesOfferWhatTheMatcherPassedOver(t *testing.T) {
	service, root, _ := newMatchTestService(t)
	writeFile(t, root, "Le film (1999).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	film := scannedFilm(t, service)

	candidates, err := service.MovieCandidates(t.Context(), film.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("offered %d candidates, want 2", len(candidates))
	}
	// Ranked the way the matcher ranks, so the film it chose leads and the
	// alternative is visible right under it rather than buried.
	if candidates[0].TMDBID != 1 || candidates[1].TMDBID != 2 {
		t.Errorf("candidate order = %d, %d; want 1, 2", candidates[0].TMDBID, candidates[1].TMDBID)
	}
	if candidates[1].Year != 1999 || candidates[1].PosterPath != "/right.jpg" {
		t.Errorf("candidate 2 = %+v, want a year and a poster to recognise it by", candidates[1])
	}
}

func TestCorrectingAMatchNeedsAMetadataSource(t *testing.T) {
	service, root := newTestService(t) // no TMDB client
	writeFile(t, root, "Le film (1999).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	film := scannedFilm(t, service)

	if _, err := service.MovieCandidates(t.Context(), film.ID, ""); err != ErrNoMetadataSource {
		t.Errorf("error = %v, want ErrNoMetadataSource", err)
	}
	if _, err := service.SetMovieMatch(t.Context(), defaultProfileID, film.ID, 2); err != ErrNoMetadataSource {
		t.Errorf("error = %v, want ErrNoMetadataSource", err)
	}
}
