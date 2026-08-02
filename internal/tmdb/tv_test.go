package tmdb

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSearchTVUsesYearAndPrefersExactOriginalName(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/tv" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("query") != "Money Heist" || query.Get("first_air_date_year") != "2017" ||
			query.Get("language") != language || query.Get("include_adult") != "false" {
			t.Errorf("query = %v", query)
		}
		w.Write([]byte(`{"results":[
			{"id":1,"name":"Money Heist: Korea","original_name":"종이의 집: 공동경제구역","popularity":99},
			{"id":2,"name":"La casa de papel","original_name":"Money Heist","popularity":10}
		]}`))
	})
	id, err := client.SearchTV(t.Context(), "Money Heist", 2017)
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Fatalf("id = %d, want exact original-name match 2", id)
	}
}

func TestSearchTVRetriesWithoutYear(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Write([]byte(`{"results":[]}`))
			return
		}
		if r.URL.Query().Get("first_air_date_year") != "" {
			t.Error("year remained on fallback request")
		}
		w.Write([]byte(`{"results":[{"id":42,"name":"Dark"}]}`))
	})
	id, err := client.SearchTV(t.Context(), "Dark", 2018)
	if err != nil || id != 42 || calls.Load() != 2 {
		t.Fatalf("id=%d calls=%d err=%v", id, calls.Load(), err)
	}
}

func TestTVDetailsAndSeasonDetailsExtractOnlyUsefulFields(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tv/66732":
			if !strings.Contains(r.URL.RawQuery, "append_to_response=credits") {
				t.Error("series credits were not appended")
			}
			w.Write([]byte(`{
				"id":66732,"name":"Stranger Things","original_name":"Stranger Things",
				"overview":"Mystères à Hawkins","first_air_date":"2016-07-15",
				"poster_path":"/poster.jpg","backdrop_path":"/backdrop.jpg","vote_average":8.6,
				"genres":[{"name":"Drame"}],"created_by":[{"name":"The Duffer Brothers"}],
				"credits":{"cast":[{"name":"Winona Ryder","character":"Joyce Byers"}]}
			}`))
		case "/tv/66732/season/1":
			w.Write([]byte(`{
				"id":77680,"season_number":1,"name":"Saison 1","overview":"Le début",
				"air_date":"2016-07-15","poster_path":"/s1.jpg",
				"episodes":[
					{"id":120000,"episode_number":1,"name":"Chapitre Un","overview":"Disparition",
					 "air_date":"2016-07-15","still_path":"/e1.jpg","runtime":49,"vote_average":8.4}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	})
	series, err := client.TVDetails(t.Context(), 66732)
	if err != nil {
		t.Fatal(err)
	}
	if series.Name != "Stranger Things" || len(series.Creators) != 1 ||
		len(series.Cast) != 1 || series.Cast[0].Character != "Joyce Byers" {
		t.Fatalf("series = %+v", series)
	}
	season, err := client.TVSeasonDetails(t.Context(), 66732, 1)
	if err != nil {
		t.Fatal(err)
	}
	if season.TMDBID != 77680 || season.EpisodeCount != 1 ||
		season.Episodes[0].RuntimeMinutes != 49 || season.Episodes[0].Name != "Chapitre Un" {
		t.Fatalf("season = %+v", season)
	}
}
