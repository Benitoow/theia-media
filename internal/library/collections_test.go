package library

import (
	"testing"
	"time"

	"github.com/Benitoow/theia-media/internal/tmdb"
)

// saveFilm writes one TMDB record onto a scanned row, the way a scan would.
func saveFilm(t *testing.T, s *Service, id int64, film *tmdb.Film) {
	t.Helper()
	if err := s.store.SaveMetadata(t.Context(), id, film, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestCollectionPartsListsWhatTheLibraryHoldsInReleaseOrder(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Le Parrain (1972).mkv")
	writeFile(t, root, "Le Parrain 2 (1974).mkv")
	writeFile(t, root, "Heat (1995).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	movies, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]int64{}
	for _, m := range movies {
		byTitle[m.Title] = m.ID
	}

	saga := &tmdb.Collection{TMDBID: 230, Name: "Trilogie Le Parrain"}
	// Deliberately out of release order, and the second part is written first:
	// the row must be ordered by the film, not by the row id.
	saveFilm(t, service, byTitle["Le Parrain 2"], &tmdb.Film{
		TMDBID: 240, Title: "Le Parrain 2", ReleaseDate: "1974-12-20", Collection: saga,
	})
	saveFilm(t, service, byTitle["Le Parrain"], &tmdb.Film{
		TMDBID: 238, Title: "Le Parrain", ReleaseDate: "1972-03-14", Collection: saga,
	})
	saveFilm(t, service, byTitle["Heat"], &tmdb.Film{
		TMDBID: 949, Title: "Heat", ReleaseDate: "1995-12-15",
	})

	first, err := service.store.Get(t.Context(), defaultProfileID, byTitle["Le Parrain"])
	if err != nil {
		t.Fatal(err)
	}
	if first.Metadata.Collection == nil || first.Metadata.Collection.Name != "Trilogie Le Parrain" {
		t.Fatalf("collection = %+v", first.Metadata.Collection)
	}
	if len(first.CollectionParts) != 1 {
		t.Fatalf("parts = %d, want only the other film of the saga", len(first.CollectionParts))
	}
	// The film being looked at is not one of its own siblings: the row sits
	// under its title.
	if first.CollectionParts[0].ID == first.ID {
		t.Error("the film lists itself as part of its own collection")
	}
	if got := first.CollectionParts[0].Metadata.Title; got != "Le Parrain 2" {
		t.Errorf("part = %q", got)
	}
	// Part three is not on the disk, and TMDB is not asked to advertise it.
	for _, part := range first.CollectionParts {
		if part.Metadata.Title == "Le Parrain 3" {
			t.Error("a film the library does not hold reached the collection row")
		}
	}

	// A film in no collection carries no row at all, rather than an empty one.
	heat, err := service.store.Get(t.Context(), defaultProfileID, byTitle["Heat"])
	if err != nil {
		t.Fatal(err)
	}
	if heat.Metadata.Collection != nil || len(heat.CollectionParts) != 0 {
		t.Errorf("Heat has a collection: %+v / %d parts", heat.Metadata.Collection, len(heat.CollectionParts))
	}
}

func TestCollectionPartsAreNotCarriedByListedFilms(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Alien (1979).mkv")
	writeFile(t, root, "Aliens (1986).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	saga := &tmdb.Collection{TMDBID: 8091, Name: "Alien"}
	for i, m := range movies {
		saveFilm(t, service, m.ID, &tmdb.Film{
			TMDBID: 100 + i, Title: m.Title, ReleaseDate: "1979-05-25", Collection: saga,
		})
	}

	listed, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	// The library page asks for a hundred films at a time. Each one dragging its
	// siblings along would send the same rows several times over.
	for _, m := range listed {
		if len(m.CollectionParts) != 0 {
			t.Errorf("%q carried %d collection parts in a list", m.Title, len(m.CollectionParts))
		}
	}
}

// The saved record must survive the round trip whole: a field written and not
// read back is the failure this whole change is about.
func TestSavedEnrichmentIsReadBack(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Blade Runner 2049 (2017).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	id := movies[0].ID

	saveFilm(t, service, id, &tmdb.Film{
		TMDBID:               335984,
		Title:                "Blade Runner 2049",
		OriginalTitle:        "Blade Runner 2049",
		OriginalLanguage:     "en",
		Tagline:              "Le futur est un souvenir",
		Certification:        "12",
		CertificationCountry: "FR",
		Director:             "Denis Villeneuve",
		Cast: []tmdb.Person{
			{Name: "Ryan Gosling", Character: "K", ProfilePath: "/gosling.jpg"},
		},
		Crew: []tmdb.Person{
			{Name: "Hampton Fancher", Role: tmdb.RoleWriting},
			{Name: "Roger Deakins", Role: tmdb.RoleCinematography, ProfilePath: "/deakins.jpg"},
		},
	})

	film, err := service.store.Get(t.Context(), defaultProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	meta := film.Metadata
	if meta.Tagline != "Le futur est un souvenir" {
		t.Errorf("tagline = %q", meta.Tagline)
	}
	if meta.OriginalTitle != "Blade Runner 2049" || meta.OriginalLanguage != "en" {
		t.Errorf("original = %q / %q", meta.OriginalTitle, meta.OriginalLanguage)
	}
	if meta.Certification != "12" || meta.CertificationCountry != "FR" {
		t.Errorf("certificate = %q (%q)", meta.Certification, meta.CertificationCountry)
	}
	if len(meta.Cast) != 1 || meta.Cast[0].ProfilePath != "/gosling.jpg" {
		t.Errorf("cast = %+v", meta.Cast)
	}
	if len(meta.Crew) != 2 {
		t.Fatalf("crew = %+v", meta.Crew)
	}
	if meta.Crew[0].Role != tmdb.RoleWriting || meta.Crew[1].Name != "Roger Deakins" {
		t.Errorf("crew = %+v", meta.Crew)
	}
}

// The backfill: a row written by an older field set is stale on its own, even
// though its status is 'ok' and it was fetched a minute ago.
func TestARowWrittenByAnOlderFieldSetIsStale(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Solaris (1972).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	id := movies[0].ID
	now := time.Now()

	saveFilm(t, service, id, &tmdb.Film{TMDBID: 393, Title: "Solaris"})

	// Freshly written by the current code: nothing to do.
	stale, err := service.store.StaleMetadata(t.Context(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 0 {
		t.Fatalf("a freshly enriched film came back stale: %+v", stale)
	}

	// Now make it look like a row from before the enrichment.
	if _, err := service.store.db.ExecContext(t.Context(),
		`UPDATE movies SET metadata_version = 0 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}

	stale, err = service.store.StaleMetadata(t.Context(), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || stale[0].ID != id {
		t.Fatalf("stale = %+v, want the film written by the older field set", stale)
	}
}

// The wire rule: a list carries what a card reads, a detail carries the record.
// Measured on 250 films, the cast and the crew alone were 31% of the list
// response, for fields no list view has ever read.
func TestListsDoNotCarryTheDetailRecord(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Stalker (1979).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	id := movies[0].ID

	saveFilm(t, service, id, &tmdb.Film{
		TMDBID:               393,
		Title:                "Stalker",
		Tagline:              "La Zone ne rend rien",
		Overview:             "Un synopsis.",
		Certification:        "12",
		CertificationCountry: "FR",
		Director:             "Andreï Tarkovski",
		Genres:               []string{"Science-fiction"},
		VoteAverage:          8.1,
		Runtime:              161,
		PosterPath:           "/poster.jpg",
		BackdropPath:         "/backdrop.jpg",
		Collection:           &tmdb.Collection{TMDBID: 1, Name: "Un cycle"},
		Cast:                 []tmdb.Person{{Name: "Alexandre Kaïdanovski", ProfilePath: "/a.jpg"}},
		Crew:                 []tmdb.Person{{Name: "Arseni Tarkovski", Role: tmdb.RoleMusic}},
	})

	listed, err := service.store.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	meta := listed[0].Metadata
	for name, present := range map[string]bool{
		"cast":          len(meta.Cast) > 0,
		"crew":          len(meta.Crew) > 0,
		"tagline":       meta.Tagline != "",
		"certification": meta.Certification != "",
		"collection":    meta.Collection != nil,
	} {
		if present {
			t.Errorf("a listed film carries its %s", name)
		}
	}
	// And everything a card, a filter or a sort reads is still there.
	for name, missing := range map[string]bool{
		"title":    meta.Title == "",
		"poster":   meta.PosterPath == "",
		"backdrop": meta.BackdropPath == "",
		"overview": meta.Overview == "",
		"director": meta.Director == "",
		"genres":   len(meta.Genres) == 0,
		"rating":   meta.VoteAverage == 0,
		"runtime":  meta.Runtime == 0,
	} {
		if missing {
			t.Errorf("a listed film lost its %s, which the library page reads", name)
		}
	}

	// The detail read is untouched.
	film, err := service.store.Get(t.Context(), defaultProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(film.Metadata.Cast) == 0 || film.Metadata.Tagline == "" || film.Metadata.Collection == nil {
		t.Errorf("the detail read lost the record: %+v", film.Metadata)
	}
}
