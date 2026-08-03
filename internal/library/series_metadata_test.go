package library

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/tmdb"
)

func TestSeriesScanFetchesOnlyLocalSeasonsAndCachesMetadata(t *testing.T) {
	var calls atomic.Int32
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/search/tv":
			if r.URL.Query().Get("first_air_date_year") != "2022" {
				t.Errorf("search year = %q", r.URL.Query().Get("first_air_date_year"))
			}
			w.Write([]byte(`{"results":[{"id":95396,"name":"Severance","original_name":"Severance"}]}`))
		case "/tv/95396":
			w.Write([]byte(`{
				"id":95396,"name":"Severance","original_name":"Severance",
				"overview":"Une séparation radicale","first_air_date":"2022-02-18",
				"poster_path":"/poster.jpg","backdrop_path":"/backdrop.jpg","vote_average":8.4,
				"genres":[{"name":"Drame"}],"created_by":[{"name":"Dan Erickson"}],
				"credits":{"cast":[{"name":"Adam Scott","character":"Mark Scout"}]}
			}`))
		case "/tv/95396/season/1":
			w.Write([]byte(`{
				"id":141759,"season_number":1,"name":"Saison 1","episode_count":9,
				"episodes":[
					{"id":2708354,"episode_number":1,"name":"Bonne nouvelle pour l'enfer",
					 "overview":"Premier épisode","air_date":"2022-02-18","still_path":"/still.jpg",
					 "runtime":57,"vote_average":8.1},
					{"id":2708355,"episode_number":2,"name":"Demi-boucle",
					 "overview":"Deuxième épisode","air_date":"2022-02-25","still_path":"/still2.jpg",
					 "runtime":54,"vote_average":8.0}
				]
			}`))
		default:
			t.Fatalf("unexpected TMDB path %q", r.URL.Path)
		}
	}))
	t.Cleanup(stub.Close)

	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	client := tmdb.New("token", tmdb.WithBaseURL(stub.URL), tmdb.WithRequestInterval(0))
	service := NewService(NewStore(database), client, slog.New(slog.DiscardHandler))
	root := t.TempDir()
	writeFile(t, root, "Severance (2022)/Season 1/S01E01.mkv")

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Enriched != 1 || report.MetadataErrors != 0 || calls.Load() != 3 {
		t.Fatalf("report=%+v calls=%d, want one series and three TMDB calls", report, calls.Load())
	}
	series := onlySeries(t, service)
	detail, err := service.GetSeries(t.Context(), defaultProfileID, series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Metadata.TMDBID != 95396 || detail.Metadata.Name != "Severance" ||
		len(detail.Metadata.Cast) != 1 || detail.Metadata.Creators[0] != "Dan Erickson" {
		t.Fatalf("series metadata = %+v", detail.Metadata)
	}
	season, err := service.GetSeason(t.Context(), defaultProfileID, series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if season.Metadata.TMDBID != 141759 || season.Items[0].Episodes[0].Metadata.TMDBID != 2708354 ||
		season.Items[0].Episodes[0].Metadata.RuntimeMinutes != 57 {
		t.Fatalf("season metadata = %+v items=%+v", season.Metadata, season.Items)
	}

	before := calls.Load()
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before {
		t.Fatalf("fresh metadata made %d extra calls", calls.Load()-before)
	}

	writeFile(t, root, "Severance (2022)/Season 1/S01E02.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != before+2 {
		t.Fatalf("new local episode made %d calls, want series detail plus local season detail",
			calls.Load()-before)
	}
	season, err = service.GetSeason(t.Context(), defaultProfileID, series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Items) != 2 || season.Items[1].Episodes[0].Metadata.TMDBID != 2708355 ||
		season.Items[1].Episodes[0].Metadata.Status != statusOK {
		t.Fatalf("new episode metadata = %+v", season.Items)
	}
}
