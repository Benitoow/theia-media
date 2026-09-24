package library

import (
	"testing"
	"time"
)

func watchedSeriesNamed(t *testing.T, service *Service, name string) Series {
	t.Helper()
	series, err := service.ListSeries(t.Context(), 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, one := range series {
		if one.Title == name {
			return one
		}
	}
	t.Fatalf("no series named %q in %v", name, series)
	return Series{}
}

func TestWatchStatsCountsWhatWasWatched(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Ran.1985.1080p.mkv")
	writeFile(t, root, "Star.Wars.1977.2160p.mkv")
	writeFile(t, root, "Solaris.1972.mkv")
	writeFile(t, root, "Shogun/Season 1/S01E01.mkv")
	writeFile(t, root, "Shogun/Season 1/S01E02.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	films, err := service.List(t.Context(), defaultProfileID, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	byTitle := map[string]Movie{}
	for _, film := range films {
		byTitle[film.Title] = film
	}

	// 7100 of 7200 leaves 100 seconds, inside the two-minute window, so this
	// one is finished by the player's own rule.
	if _, err := service.SaveProgress(t.Context(), defaultProfileID,
		byTitle["Ran"].ID, 7100, 7200); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveProgress(t.Context(), defaultProfileID,
		byTitle["Star Wars"].ID, 1800, 7200); err != nil {
		t.Fatal(err)
	}
	// Opened to look at the poster and closed again: below the floor, so it is
	// not remembered and must not count as started.
	if _, err := service.SaveProgress(t.Context(), defaultProfileID,
		byTitle["Solaris"].ID, 5, 7200); err != nil {
		t.Fatal(err)
	}

	series := watchedSeriesNamed(t, service, "Shogun")
	season, err := service.GetSeason(t.Context(), defaultProfileID, series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Items) != 2 {
		t.Fatalf("season items = %d, want 2", len(season.Items))
	}
	if _, err := service.SaveEpisodeProgress(t.Context(), defaultProfileID,
		season.Items[0].ID, 1190, 1200); err != nil {
		t.Fatal(err)
	}

	stats, err := service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Movies.Started != 2 || stats.Movies.Finished != 1 || stats.Movies.Seconds != 8900 {
		t.Errorf("movies = %+v, want 2 started, 1 finished, 8900 seconds", stats.Movies)
	}
	if stats.Episodes.Started != 1 || stats.Episodes.Finished != 1 || stats.Episodes.Seconds != 1190 {
		t.Errorf("episodes = %+v, want 1 started, 1 finished, 1190 seconds", stats.Episodes)
	}
	// One episode of two is finished, so the series itself is not: a series is
	// watched when every file in it is, including the ones never opened.
	if stats.Series.Started != 1 || stats.Series.Finished != 0 || stats.Series.Seconds != 1190 {
		t.Errorf("series = %+v, want 1 started, 0 finished, 1190 seconds", stats.Series)
	}
	if len(stats.Top) != 1 || stats.Top[0].ID != series.ID ||
		stats.Top[0].Episodes != 1 || stats.Top[0].Finished != 1 ||
		stats.Top[0].Total != 2 || stats.Top[0].Seconds != 1190 {
		t.Errorf("top series = %+v, want one of two episodes for series %d", stats.Top, series.ID)
	}
	// The picture travels with the row, because the ranking is drawn as the
	// series rather than as its name: a caller that had to fetch each series to
	// find its poster would be doing the join the ranking already did.
	if _, err := service.store.db.Exec(
		`UPDATE series SET poster_path = ? WHERE id = ?`, "/poster.jpg", series.ID); err != nil {
		t.Fatal(err)
	}
	stats, err = service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Top) != 1 || stats.Top[0].Poster != "/poster.jpg" {
		t.Errorf("top series = %+v, want the poster path it was given", stats.Top)
	}
}

func TestASeriesIsWatchedWhenEveryFileInItIs(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Shogun/Season 1/S01E01.mkv")
	writeFile(t, root, "Shogun/Season 1/S01E02E03.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	series := onlySeries(t, service)
	season, err := service.GetSeason(t.Context(), defaultProfileID, series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range season.Items {
		if _, err := service.SaveEpisodeProgress(t.Context(), defaultProfileID,
			item.ID, 1190, 1200); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Series.Finished != 1 {
		t.Errorf("series finished = %d, want 1 once every file is", stats.Series.Finished)
	}
	// Two files, and the second holds two episodes: the numbers count episodes,
	// which is what "how much of this have I seen" means to a viewer, so the
	// total is three rather than two.
	if stats.Episodes.Started != 3 || stats.Episodes.Finished != 3 {
		t.Errorf("episodes = %+v, want 3 of 3", stats.Episodes)
	}
	if len(stats.Top) != 1 || stats.Top[0].Episodes != 3 || stats.Top[0].Finished != 3 ||
		stats.Top[0].Total != 3 {
		t.Errorf("top series = %+v, want 3 episodes watched of 3", stats.Top)
	}

	// The rule is recomputed rather than latched, exactly as it is for films, so
	// rewatching an episode puts the series back among the unfinished.
	if err := service.ResetEpisodeProgress(t.Context(), defaultProfileID, season.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	stats, err = service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Series.Finished != 0 {
		t.Errorf("series finished = %d after rewinding an episode, want 0", stats.Series.Finished)
	}
}

func TestTheRankingIsByTimeAndBelongsToItsProfile(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Shows/Alpha/Season 1/S01E01.mkv")
	writeFile(t, root, "Shows/Beta/Season 1/S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	for name, seconds := range map[string]float64{"Alpha": 600, "Beta": 1800} {
		series := watchedSeriesNamed(t, service, name)
		season, err := service.GetSeason(t.Context(), defaultProfileID, series.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.SaveEpisodeProgress(t.Context(), defaultProfileID,
			season.Items[0].ID, seconds, 3600); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Top) != 2 || stats.Top[0].Title != "Beta" || stats.Top[1].Title != "Alpha" {
		t.Fatalf("ranking = %+v, want Beta before Alpha", stats.Top)
	}
	if stats.Series.Started != 2 {
		t.Errorf("series started = %d, want 2", stats.Series.Started)
	}

	// The caller's limit is the caller's, and it is a limit on how many lines
	// come back rather than on what was watched.
	stats, err = service.WatchStats(t.Context(), defaultProfileID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Top) != 1 || stats.Top[0].Title != "Beta" {
		t.Errorf("limited ranking = %+v, want Beta alone", stats.Top)
	}

	// A second viewer's evening is their own: the numbers are per profile, so a
	// profile that watched nothing reports nothing rather than the household's.
	if _, err := service.store.db.Exec("INSERT INTO profiles (id,name) VALUES (2,'Other')"); err != nil {
		t.Fatal(err)
	}
	other, err := service.WatchStats(t.Context(), 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if other.Movies.Started != 0 || other.Episodes.Started != 0 ||
		other.Series.Started != 0 || len(other.Top) != 0 {
		t.Errorf("another profile's stats = %+v, want nothing watched", other)
	}
}

func TestThisMonthCountsOnlyThisMonth(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Ran.1985.1080p.mkv")
	writeFile(t, root, "Shogun/Season 1/S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	films, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil || len(films) != 1 {
		t.Fatalf("films=%v err=%v", films, err)
	}
	series := onlySeries(t, service)
	season, err := service.GetSeason(t.Context(), defaultProfileID, series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	// The store takes the report's own timestamp, which is the only way to put a
	// position in last month: the service uses the clock, and that is the point
	// of the parameter.
	lastMonth := time.Now().AddDate(0, -1, 0)
	if _, err := service.store.SaveProgress(t.Context(), defaultProfileID,
		films[0].ID, 600, 7200, lastMonth); err != nil {
		t.Fatal(err)
	}
	if _, err := service.store.SaveEpisodeProgress(t.Context(), defaultProfileID,
		season.Items[0].ID, 300, 1200, lastMonth); err != nil {
		t.Fatal(err)
	}

	stats, err := service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Month != (ThisMonth{}) {
		t.Errorf("this month = %+v, want nothing watched in it", stats.Month)
	}
	// The totals are not the month: last month's evening still counts as
	// watched, and the window must not have been applied to both.
	if stats.Movies.Started != 1 || stats.Episodes.Started != 1 {
		t.Errorf("totals = %+v / %+v, want last month's watching counted", stats.Movies, stats.Episodes)
	}

	// Watching again this month moves the report, and the film counts once.
	if _, err := service.SaveProgress(t.Context(), defaultProfileID, films[0].ID, 900, 7200); err != nil {
		t.Fatal(err)
	}
	stats, err = service.WatchStats(t.Context(), defaultProfileID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Month.Movies != 1 || stats.Month.Episodes != 0 || stats.Month.Seconds != 900 {
		t.Errorf("this month = %+v, want one film of 900 seconds and no episode", stats.Month)
	}
}
