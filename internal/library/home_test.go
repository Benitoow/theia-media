package library

import (
	"fmt"
	"strings"
	"testing"
)

// dressForScreen gives a scanned row the artwork and synopsis that the hero
// queries insist on. Scanned test files carry no metadata, and both Hero and
// ResumeHero refuse a film that would leave a hole at the top of the screen.
func dressForScreen(t *testing.T, s *Service, id int64, rating float64) {
	t.Helper()
	_, err := s.store.db.ExecContext(t.Context(), `
		UPDATE movies
		SET backdrop_path = '/backdrop.jpg',
		    poster_path   = '/poster.jpg',
		    overview      = 'Un synopsis.',
		    vote_average  = ?
		WHERE id = ?`, rating, id)
	if err != nil {
		t.Fatal(err)
	}
}

func rowKinds(home *Home) []string {
	kinds := make([]string, 0, len(home.Rows))
	for _, r := range home.Rows {
		kinds = append(kinds, r.Kind)
	}
	return kinds
}

func TestHomeHeroPrefersWhatIsAlreadyUnderway(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Stalker (1979).mkv")
	writeFile(t, root, "Solaris (1972).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	movies, err := service.store.List(t.Context(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range movies {
		dressForScreen(t, service, m.ID, 8.0)
	}

	// Nothing watched yet: the hero is the featured pick.
	home, err := service.HomeScreen(t.Context(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if home.HeroKind != HeroFeatured {
		t.Errorf("hero kind = %q, want %q before anything is watched", home.HeroKind, HeroFeatured)
	}

	// Start one of them, and it takes the slot regardless of rating or recency.
	started := movies[1].ID
	if _, err := service.SaveProgress(t.Context(), started, 1800, 10140); err != nil {
		t.Fatal(err)
	}

	home, err = service.HomeScreen(t.Context(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if home.HeroKind != HeroResume {
		t.Fatalf("hero kind = %q, want %q once a film is in progress", home.HeroKind, HeroResume)
	}
	if home.Hero == nil || home.Hero.ID != started {
		t.Errorf("hero = %v, want the film in progress (%d)", home.Hero, started)
	}
}

func TestHomeRowsAreCodesNotSentences(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Stalker (1979).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID
	dressForScreen(t, service, id, 8.0)
	if _, err := service.SaveProgress(t.Context(), id, 1800, 10140); err != nil {
		t.Fatal(err)
	}

	home, err := service.HomeScreen(t.Context(), 12)
	if err != nil {
		t.Fatal(err)
	}

	// Decision 25: the server sends codes, the interface writes the sentences.
	// This row set used to arrive with "Continuer à regarder" already in it.
	want := map[string]bool{RowContinue: true, RowRecent: true, RowTopRated: true, RowTonight: true}
	for _, kind := range rowKinds(home) {
		if !want[kind] {
			t.Errorf("unexpected row kind %q", kind)
		}
		if strings.ContainsAny(kind, "éàèÉÀÈ ") {
			t.Errorf("row kind %q looks like display text, not a code", kind)
		}
	}
	if got := rowKinds(home); len(got) == 0 {
		t.Fatal("no rows at all")
	}
	if home.Rows[0].Kind != RowContinue {
		t.Errorf("first row = %q, want %q: resuming is the reason the screen exists",
			home.Rows[0].Kind, RowContinue)
	}
}

func TestTonightHoldsForTheEveningAndTurnsOver(t *testing.T) {
	service, root := newTestService(t)
	for i := range 12 {
		writeFile(t, root, fmt.Sprintf("Film %02d (2020).mkv", i))
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range movies {
		dressForScreen(t, service, m.ID, 7.0)
	}

	order := func(seed int64) string {
		picks, err := service.store.Tonight(t.Context(), 12, seed)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, p := range picks {
			fmt.Fprintf(&b, "%d,", p.ID)
		}
		return b.String()
	}

	// The whole point of seeding rather than ORDER BY RANDOM(): reloading the
	// page must not reshuffle, or the suggestion means nothing.
	if a, b := order(20260729), order(20260729); a != b {
		t.Errorf("same seed gave two orders:\n %s\n %s", a, b)
	}

	// And it has to actually turn over, otherwise it is just a fixed list.
	seen := map[string]bool{}
	for _, seed := range []int64{20260729, 20260730, 20260731, 20260801} {
		seen[order(seed)] = true
	}
	if len(seen) < 2 {
		t.Error("every day produced the same order; the seed is not reaching the query")
	}
}

// The first two shuffles were stable and turned over daily, and were still
// wrong: sorting by (id * k) mod p and taking the first twelve picks ids at a
// fixed stride. On the real library the row came back as 257, 234, 211, 188 …,
// every twenty-third film. It passed every other test here.
func TestTonightIsNotAnArithmeticProgression(t *testing.T) {
	service, root := newTestService(t)
	for i := range 60 {
		writeFile(t, root, fmt.Sprintf("Film %02d (2020).mkv", i))
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range movies {
		dressForScreen(t, service, m.ID, 7.0)
	}

	picks, err := service.store.Tonight(t.Context(), 12, 20260729)
	if err != nil {
		t.Fatal(err)
	}
	if len(picks) < 3 {
		t.Fatalf("got %d picks, need at least 3 to judge the spacing", len(picks))
	}

	gaps := map[int64]int{}
	for i := 1; i < len(picks); i++ {
		gaps[picks[i].ID-picks[i-1].ID]++
	}
	for gap, n := range gaps {
		if n == len(picks)-1 {
			t.Errorf("every pick is %d apart: this is a stride, not a shuffle", gap)
		}
	}
}

func TestTonightLeavesOutWhatIsAlreadyFinished(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Stalker (1979).mkv")
	writeFile(t, root, "Solaris (1972).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.store.List(t.Context(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range movies {
		dressForScreen(t, service, m.ID, 7.0)
	}

	// The finished rule wants the last min(120s, 5%) of the film, so for a
	// 10140-second one that is anything past 10020. 10000 would still count as
	// in progress, which is what this test asserted on its first run.
	finished := movies[0].ID
	if _, err := service.SaveProgress(t.Context(), finished, 10100, 10140); err != nil {
		t.Fatal(err)
	}

	picks, err := service.store.Tonight(t.Context(), 12, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range picks {
		if p.ID == finished {
			t.Error("tonight suggested a film that has already been watched to the end")
		}
	}
}
