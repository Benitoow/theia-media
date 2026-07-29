package library

import (
	"testing"
	"time"

	"github.com/Benitoow/theia-media/internal/profile"
)

func TestFinishedRule(t *testing.T) {
	tests := []struct {
		name               string
		position, duration float64
		want               bool
	}{
		{"not started", 0, 7200, false},
		{"a third in", 2400, 7200, false},
		{"most of the way", 6000, 7200, false},
		// A three-hour epic clears at two minutes from the end, not at five
		// per cent of it -- nine minutes of credits is not a viewing session.
		{"two minutes left of three hours", 10680, 10800, true},
		{"nine minutes left of three hours", 10260, 10800, false},
		// Credits run longer than two minutes, so stopping when they start
		// still counts as having watched the film.
		{"two minutes left", 7080, 7200, true},
		{"right at the end", 7200, 7200, true},
		{"past the end", 7300, 7200, true},
		// The fraction is what catches short films, where two minutes is a
		// large part of the running time.
		{"95 per cent of a short film", 570, 600, true},
		{"90 per cent of a short film", 540, 600, false},
		// Without a duration there is nothing to be near the end of.
		{"unknown duration", 5000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := finishedRule(tt.position, tt.duration); got != tt.want {
				t.Errorf("finishedRule(%v, %v) = %v, want %v",
					tt.position, tt.duration, got, tt.want)
			}
		})
	}
}

func TestSaveProgressIgnoresAGlance(t *testing.T) {
	// Opening a film, looking at the poster and closing it must not fill the
	// continue-watching row.
	service, root := newTestService(t)
	writeFile(t, root, "Alien (1979).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	got, err := service.SaveProgress(t.Context(), testProfileID, id, 12, 7200)
	if err != nil {
		t.Fatal(err)
	}
	if got.PositionSeconds != 0 {
		t.Errorf("position = %v, want 0 for a glance", got.PositionSeconds)
	}

	rows, err := service.store.ContinueWatching(t.Context(), testProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("continue watching holds %d films, want none", len(rows))
	}
}

func TestProgressRoundTripsAndAppearsInTheRow(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Solaris (1972).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 1800, 10140); err != nil {
		t.Fatal(err)
	}

	movie, err := service.Get(t.Context(), testProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	if movie.Progress.PositionSeconds != 1800 {
		t.Errorf("position = %v, want 1800", movie.Progress.PositionSeconds)
	}
	if movie.Progress.DurationSeconds != 10140 {
		t.Errorf("duration = %v, want 10140", movie.Progress.DurationSeconds)
	}
	if movie.Progress.Finished {
		t.Error("a film 18 per cent in is not finished")
	}
	if movie.Progress.WatchedAt == nil {
		t.Error("watched_at was not recorded")
	}

	rows, err := service.store.ContinueWatching(t.Context(), testProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != id {
		t.Errorf("continue watching = %d rows, want the one film", len(rows))
	}
}

func TestFinishingRemovesAFilmFromTheRow(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Stalker (1979).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 3000, 9660); err != nil {
		t.Fatal(err)
	}
	if rows, _ := service.store.ContinueWatching(t.Context(), testProfileID, 10); len(rows) != 1 {
		t.Fatalf("expected the film to be in the row before finishing")
	}

	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 9660, 9660); err != nil {
		t.Fatal(err)
	}
	rows, err := service.store.ContinueWatching(t.Context(), testProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("a finished film is still in the continue-watching row")
	}
}

func TestRewatchingBringsAFilmBack(t *testing.T) {
	// Finished is recomputed on every report rather than latched, so starting
	// a film again puts it back where somebody rewatching expects it.
	service, root := newTestService(t)
	writeFile(t, root, "Ran (1985).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 9000, 9000); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 300, 9000); err != nil {
		t.Fatal(err)
	}

	rows, err := service.store.ContinueWatching(t.Context(), testProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("a rewatched film did not come back to the row")
	}
}

func TestResetProgressStartsFromTheBeginning(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Ikiru (1952).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 2000, 8400); err != nil {
		t.Fatal(err)
	}
	if err := service.ResetProgress(t.Context(), testProfileID, id); err != nil {
		t.Fatal(err)
	}

	movie, err := service.Get(t.Context(), testProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	if movie.Progress.PositionSeconds != 0 || movie.Progress.WatchedAt != nil {
		t.Errorf("progress = %+v, want it forgotten", movie.Progress)
	}
	// The duration describes the file, not the viewing, so it survives.
	if movie.Progress.DurationSeconds != 8400 {
		t.Errorf("duration = %v, want it kept", movie.Progress.DurationSeconds)
	}
}

func TestContinueWatchingIsOrderedByMostRecent(t *testing.T) {
	service, root := newTestService(t)
	for _, name := range []string{"A (2001).mkv", "B (2002).mkv", "C (2003).mkv"} {
		writeFile(t, root, name)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), testProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Watched oldest first; the row must come back newest first. Timestamps are
	// set explicitly because three saves in the same second would otherwise be
	// indistinguishable -- the same trap as decisions 7c and 13.
	base := time.Now().Add(-time.Hour)
	for i, m := range movies {
		if _, err := service.store.SaveProgress(t.Context(), testProfileID, m.ID, 600, 7200,
			base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := service.store.ContinueWatching(t.Context(), testProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0].ID != movies[2].ID || rows[2].ID != movies[0].ID {
		t.Errorf("row order = %v, want most recently watched first",
			[]int64{rows[0].ID, rows[1].ID, rows[2].ID})
	}
}

func TestPlaybackProgressIsSeparatedByProfile(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Arrival (2016).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	second, err := profile.NewStore(service.store.db).Create(t.Context(), "Sam")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 1200, 6960); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveProgress(t.Context(), second.ID, id, 2400, 0); err != nil {
		t.Fatal(err)
	}

	firstMovie, err := service.Get(t.Context(), testProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	secondMovie, err := service.Get(t.Context(), second.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	if firstMovie.Progress.PositionSeconds != 1200 {
		t.Errorf("default position = %v, want 1200", firstMovie.Progress.PositionSeconds)
	}
	if secondMovie.Progress.PositionSeconds != 2400 {
		t.Errorf("second position = %v, want 2400", secondMovie.Progress.PositionSeconds)
	}
	if firstMovie.Progress.DurationSeconds != 6960 || secondMovie.Progress.DurationSeconds != 6960 {
		t.Errorf("duration is not shared: default=%v second=%v, want 6960",
			firstMovie.Progress.DurationSeconds, secondMovie.Progress.DurationSeconds)
	}

	firstRows, err := service.store.ContinueWatching(t.Context(), testProfileID, 10)
	if err != nil {
		t.Fatal(err)
	}
	secondRows, err := service.store.ContinueWatching(t.Context(), second.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstRows) != 1 || firstRows[0].Progress.PositionSeconds != 1200 {
		t.Errorf("default continue row = %+v, want its own progress", firstRows)
	}
	if len(secondRows) != 1 || secondRows[0].Progress.PositionSeconds != 2400 {
		t.Errorf("second continue row = %+v, want its own progress", secondRows)
	}
}

func TestLegacyProgressBridgeKeepsRollbackCompatible(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Moon (2009).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	id := onlyMovie(t, service).ID

	// New default-profile writes remain visible to the v1.2 columns.
	if _, err := service.SaveProgress(t.Context(), testProfileID, id, 900, 5820); err != nil {
		t.Fatal(err)
	}
	var legacyPosition float64
	if err := service.store.db.QueryRowContext(t.Context(),
		`SELECT position_seconds FROM movies WHERE id = ?`, id).Scan(&legacyPosition); err != nil {
		t.Fatal(err)
	}
	if legacyPosition != 900 {
		t.Errorf("legacy position = %v, want 900", legacyPosition)
	}

	// An old executable writing after a rollback is mirrored back into the
	// default profile by the migration trigger.
	if _, err := service.store.db.ExecContext(t.Context(), `
		UPDATE movies
		SET position_seconds = 1800, watched_at = unixepoch(), finished = 0
		WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	movie, err := service.Get(t.Context(), testProfileID, id)
	if err != nil {
		t.Fatal(err)
	}
	if movie.Progress.PositionSeconds != 1800 {
		t.Errorf("profile position after legacy write = %v, want 1800",
			movie.Progress.PositionSeconds)
	}
}

func onlyMovie(t *testing.T, s *Service) Movie {
	t.Helper()
	movies, err := s.List(t.Context(), testProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 {
		t.Fatalf("expected exactly one film, got %d", len(movies))
	}
	return movies[0]
}
