package library

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestWatchlistIsIsolatedIdempotentAndCascades(t *testing.T) {
	s, root := newTestService(t)
	writeFile(t, root, "Arrival.2016.mkv")
	if _, err := s.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := s.List(t.Context(), 1, 10, 0)
	if err != nil || len(movies) != 1 {
		t.Fatalf("movies=%v err=%v", movies, err)
	}
	id := movies[0].ID
	if _, err := s.store.db.Exec("INSERT INTO profiles (id,name) VALUES (2,'Other')"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.SetWatchlist(t.Context(), 1, id, true); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := s.Watchlist(t.Context(), 1)
	if err != nil || !reflect.DeepEqual(ids, []int64{id}) {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	other, err := s.Watchlist(t.Context(), 2)
	if err != nil || len(other) != 0 {
		t.Fatalf("other=%v err=%v", other, err)
	}
	if err := s.SetWatchlist(t.Context(), 2, id, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWatchlist(t.Context(), 1, id, false); err != nil {
		t.Fatal(err)
	}
	other, _ = s.Watchlist(t.Context(), 2)
	if len(other) != 1 {
		t.Fatal("removing one viewer's entry changed another")
	}
	if _, err := s.store.db.Exec("DELETE FROM profiles WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.store.db.QueryRow("SELECT count(*) FROM profile_watchlist").Scan(&count); err != nil || count != 0 {
		t.Fatalf("cascade count=%d err=%v", count, err)
	}
	if err := s.SetWatchlist(t.Context(), 1, 99999, true); err == nil {
		t.Fatal("missing movie accepted")
	}
}

func TestFreshReplacementPreservesExistingMovie(t *testing.T) {
	w, s, root := newTestWatcher(t)
	path := writeFile(t, root, "Arrival.2016.mkv")
	settle(t, path)
	w.pass(t.Context(), true)
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	settle(t, writeFile(t, root, "Dune.2021.mkv"))
	w.pass(t.Context(), false)
	if got := count(t, s); got != 2 {
		t.Fatalf("fresh replacement was pruned: count=%d", got)
	}
}
