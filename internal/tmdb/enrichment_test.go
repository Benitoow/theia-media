package tmdb

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The whole point of the enrichment is that it costs no extra request: the
// certificates and the credits ride along with the details call.
func TestDetailsAsksForCreditsAndReleaseDatesInOneCall(t *testing.T) {
	var calls int
	var query url.Values

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		query = r.URL.Query()
		w.Write([]byte(`{"id":603,"title":"Matrix"}`))
	})

	if _, err := client.Details(t.Context(), 603); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("Details made %d requests, want 1", calls)
	}
	appended := query.Get("append_to_response")
	for _, wanted := range []string{"credits", "release_dates"} {
		if !strings.Contains(appended, wanted) {
			t.Errorf("append_to_response = %q, want it to include %q", appended, wanted)
		}
	}
}

func TestDetailsKeepsTheTaglineTheCollectionAndTheCertificate(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"id": 120,
			"title": "La Communauté de l'anneau",
			"original_title": "The Fellowship of the Ring",
			"original_language": "en",
			"tagline": "  Un anneau pour les gouverner tous  ",
			"belongs_to_collection": {"id": 119, "name": "Le Seigneur des anneaux", "poster_path": "/saga.jpg"},
			"release_dates": {"results": [
				{"iso_3166_1": "US", "release_dates": [{"certification": "PG-13", "type": 3}]},
				{"iso_3166_1": "FR", "release_dates": [
					{"certification": "16", "type": 5},
					{"certification": "12", "type": 3}
				]}
			]}
		}`))
	})

	film, err := client.Details(t.Context(), 120)
	if err != nil {
		t.Fatal(err)
	}
	if film.Tagline != "Un anneau pour les gouverner tous" {
		t.Errorf("Tagline = %q, want it trimmed", film.Tagline)
	}
	if film.OriginalTitle != "The Fellowship of the Ring" || film.OriginalLanguage != "en" {
		t.Errorf("original title/language = %q/%q", film.OriginalTitle, film.OriginalLanguage)
	}
	if film.Collection == nil {
		t.Fatal("Collection is nil, want the saga TMDB sent")
	}
	if film.Collection.TMDBID != 119 || film.Collection.Name != "Le Seigneur des anneaux" {
		t.Errorf("Collection = %+v", film.Collection)
	}
	// France is preferred over the United States, and within France the
	// theatrical certificate wins over the television one.
	if film.Certification != "12" || film.CertificationCountry != "FR" {
		t.Errorf("certificate = %q (%q), want 12 (FR)", film.Certification, film.CertificationCountry)
	}
}

func TestDetailsFallsBackToAnotherCountryForTheCertificate(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"id": 1,
			"release_dates": {"results": [
				{"iso_3166_1": "DE", "release_dates": [{"certification": "18", "type": 3}]},
				{"iso_3166_1": "FR", "release_dates": [{"certification": "", "type": 3}]},
				{"iso_3166_1": "US", "release_dates": [{"certification": "R", "type": 4}]}
			]}
		}`))
	})

	film, err := client.Details(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// An empty French certificate is not a French certificate, and Germany is
	// not on the preference list at all.
	if film.Certification != "R" || film.CertificationCountry != "US" {
		t.Errorf("certificate = %q (%q), want R (US)", film.Certification, film.CertificationCountry)
	}
}

func TestDetailsNoCertificateIsOrdinary(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":1,"title":"Un film sans visa"}`))
	})

	film, err := client.Details(t.Context(), 1)
	if err != nil {
		t.Fatalf("a film with no certificate must not be an error: %v", err)
	}
	if film.Certification != "" || film.CertificationCountry != "" {
		t.Errorf("certificate = %q (%q), want both empty", film.Certification, film.CertificationCountry)
	}
	if film.Collection != nil {
		t.Errorf("Collection = %+v, want nil when TMDB sends none", film.Collection)
	}
}

func TestDetailsKeepsTheCrewWorthNamingAndNoMore(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"id": 1,
			"credits": {
				"cast": [{"name": "Keanu Reeves", "character": "Neo", "profile_path": "/keanu.jpg"}],
				"crew": [
					{"name": "Lana Wachowski", "job": "Director"},
					{"name": "Lana Wachowski", "job": "Screenplay"},
					{"name": "Lilly Wachowski", "job": "Screenplay"},
					{"name": "David Mitchell", "job": "Novel"},
					{"name": "Don Davis", "job": "Original Music Composer", "profile_path": "/davis.jpg"},
					{"name": "Bill Pope", "job": "Director of Photography"},
					{"name": "Somebody", "job": "Gaffer"},
					{"name": "Somebody Else", "job": "Best Boy Electric"}
				]
			}
		}`))
	})

	film, err := client.Details(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if film.Director != "Lana Wachowski" {
		t.Errorf("Director = %q", film.Director)
	}
	if len(film.Cast) != 1 || film.Cast[0].ProfilePath != "/keanu.jpg" {
		t.Errorf("cast = %+v, want the portrait path kept", film.Cast)
	}

	byRole := map[string][]string{}
	for _, person := range film.Crew {
		byRole[person.Role] = append(byRole[person.Role], person.Name)
	}
	// The director is not repeated as a writer of their own film, so the two
	// writing credits kept are the sister and the novelist.
	writing := strings.Join(byRole[RoleWriting], ", ")
	if writing != "Lilly Wachowski, David Mitchell" {
		t.Errorf("writing = %q", writing)
	}
	if strings.Join(byRole[RoleMusic], ", ") != "Don Davis" {
		t.Errorf("music = %v", byRole[RoleMusic])
	}
	if strings.Join(byRole[RoleCinematography], ", ") != "Bill Pope" {
		t.Errorf("cinematography = %v", byRole[RoleCinematography])
	}
	// The grips and the gaffer are dropped: this page is not a call sheet.
	for _, person := range film.Crew {
		if person.Role == "" {
			t.Errorf("%q was kept with no role", person.Name)
		}
	}
}

func TestDetailsCapsOneRoleAtTwoNames(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"id": 1,
			"credits": {"crew": [
				{"name": "A", "job": "Screenplay"},
				{"name": "B", "job": "Screenplay"},
				{"name": "C", "job": "Screenplay"},
				{"name": "D", "job": "Writer"}
			]}
		}`))
	})

	film, err := client.Details(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(film.Crew) != maxCrewPerRole {
		t.Errorf("kept %d writers, want %d", len(film.Crew), maxCrewPerRole)
	}
}

func TestTVDetailsKeepsTheTaglineTheStatusAndTheRating(t *testing.T) {
	var query url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Write([]byte(`{
			"id": 1396,
			"name": "Breaking Bad",
			"tagline": "Change the equation",
			"status": "Ended",
			"last_air_date": "2013-09-29",
			"networks": [{"name": "AMC"}, {"name": "Netflix"}, {"name": "Third"}],
			"credits": {"cast": [{"name": "Bryan Cranston", "character": "Walter White", "profile_path": "/bc.jpg"}]},
			"content_ratings": {"results": [
				{"iso_3166_1": "US", "rating": "TV-MA"},
				{"iso_3166_1": "FR", "rating": "16"}
			]}
		}`))
	})

	series, err := client.TVDetails(t.Context(), 1396)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query.Get("append_to_response"), "content_ratings") {
		t.Errorf("append_to_response = %q", query.Get("append_to_response"))
	}
	if series.Tagline != "Change the equation" {
		t.Errorf("Tagline = %q", series.Tagline)
	}
	// A code, never TMDB's English label: the interface writes the sentence.
	if series.Status != SeriesEnded {
		t.Errorf("Status = %q, want %q", series.Status, SeriesEnded)
	}
	if series.LastAirDate != "2013-09-29" {
		t.Errorf("LastAirDate = %q", series.LastAirDate)
	}
	if len(series.Networks) != maxNetworks {
		t.Errorf("Networks = %v, want at most %d", series.Networks, maxNetworks)
	}
	if series.Certification != "16" || series.CertificationCountry != "FR" {
		t.Errorf("rating = %q (%q), want 16 (FR)", series.Certification, series.CertificationCountry)
	}
	if len(series.Cast) != 1 || series.Cast[0].ProfilePath != "/bc.jpg" {
		t.Errorf("cast = %+v", series.Cast)
	}
}

func TestTVDetailsDropsAStatusItDoesNotKnow(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":1,"status":"Some New TMDB Wording"}`))
	})

	series, err := client.TVDetails(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	// Passing it through would put an English string in the middle of a French
	// page, which is the fault decision 25 exists to prevent.
	if series.Status != "" {
		t.Errorf("Status = %q, want it dropped", series.Status)
	}
}
