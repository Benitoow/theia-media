package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanBuildsSeriesSeasonsCombinedEpisodesAndQualities(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Films/Arrival (2016).mkv")
	writeFile(t, root, "Shows/The Office (2005)/Season 01/S01E01.720p.mkv")
	writeFile(t, root, "Shows/The Office (2005)/Season 01/S01E01.1080p.mp4")
	writeFile(t, root, "Shows/The Office (2005)/Season 01/S01E02E03.mkv")
	writeFile(t, root, "Shows/The Office (2005)/Season 01/S01E05.mkv")
	writeFile(t, root, "Shows/The Office (2005)/Specials/S00E01.mkv")

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Found != 6 || report.MovieFiles != 1 || report.EpisodeFiles != 5 {
		t.Fatalf("scan classification = %+v", report)
	}
	if report.Added != 6 || report.Series != 1 || report.Episodes != 4 {
		t.Fatalf("scan counts = %+v", report)
	}
	if movies, err := service.List(t.Context(), 10, 0); err != nil || len(movies) != 1 {
		t.Fatalf("films = %d, err = %v, want one", len(movies), err)
	}

	seriesList, err := service.ListSeries(t.Context(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(seriesList) != 1 || seriesList[0].Title != "The Office" || seriesList[0].Year != 2005 {
		t.Fatalf("series = %+v", seriesList)
	}
	detail, err := service.GetSeries(t.Context(), seriesList[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Seasons) != 2 || detail.Seasons[0].Number != 0 || detail.Seasons[1].Number != 1 {
		t.Fatalf("seasons = %+v, want specials then season one", detail.Seasons)
	}

	season, err := service.GetSeason(t.Context(), detail.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Items) != 3 {
		t.Fatalf("season items = %d, want E1, E2-E3 and E5", len(season.Items))
	}
	first, err := service.GetEpisodeItem(t.Context(), season.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != 2 || !first.Files[0].IsPrimary || len(first.Episodes) != 1 {
		t.Fatalf("first episode = %+v", first)
	}
	if first.NextEpisodeID == nil || *first.NextEpisodeID != season.Items[1].ID || first.NextHasGap {
		t.Fatalf("next after E1 = id %v gap %v, want combined E2-E3 without gap",
			first.NextEpisodeID, first.NextHasGap)
	}
	combined, err := service.GetEpisodeItem(t.Context(), season.Items[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(combined.EpisodeNumbers) != 2 || combined.EpisodeNumbers[0] != 2 || combined.EpisodeNumbers[1] != 3 {
		t.Fatalf("combined episode numbers = %v", combined.EpisodeNumbers)
	}
	if combined.NextEpisodeID == nil || *combined.NextEpisodeID != season.Items[2].ID || !combined.NextHasGap {
		t.Fatalf("next after E2-E3 = id %v gap %v, want E5 with gap",
			combined.NextEpisodeID, combined.NextHasGap)
	}
	specials, err := service.GetSeason(t.Context(), detail.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	special, err := service.GetEpisodeItem(t.Context(), specials.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if special.NextEpisodeID != nil || special.NextHasGap {
		t.Fatalf("special unexpectedly participates in autoplay: %+v", special)
	}

	second, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if second.Added != 0 || second.Updated != 6 || second.Removed != 0 || second.Series != 1 || second.Episodes != 4 {
		t.Fatalf("idempotent scan = %+v", second)
	}
}

func TestEpisodeRenamePreservesFileIdentityAndProgress(t *testing.T) {
	service, root := newTestService(t)
	original := writeFile(t, root, "A Show/Season 1/A.Show.S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	series := onlySeries(t, service)
	season, err := service.GetSeason(t.Context(), series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	before, err := service.GetEpisodeItem(t.Context(), season.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveEpisodeProgress(t.Context(), before.ID, 300, 1200); err != nil {
		t.Fatal(err)
	}

	renamed := filepath.Join(filepath.Dir(original), "A.Show.S01E02.mkv")
	if err := os.Rename(original, renamed); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	series = onlySeries(t, service)
	season, err = service.GetSeason(t.Context(), series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Items) != 1 || season.Items[0].EpisodeNumbers[0] != 2 {
		t.Fatalf("renamed season = %+v", season.Items)
	}
	after, err := service.GetEpisodeItem(t.Context(), season.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Files[0].ID != before.Files[0].ID {
		t.Errorf("file id changed from %d to %d", before.Files[0].ID, after.Files[0].ID)
	}
	if after.Progress.PositionSeconds != 300 {
		t.Errorf("progress = %v, want 300", after.Progress.PositionSeconds)
	}
}

func TestEpisodeRenameUsesSeasonContextWhenByteIdentityIsAmbiguous(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Show/Season 1/S01E01.mp4")
	source := writeFile(t, root, "Show/Season 1/S01E02.mp4")
	writeFile(t, root, "Show/Specials/S00E01.mp4")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	series := onlySeries(t, service)
	season, err := service.GetSeason(t.Context(), series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	before, err := service.GetEpisodeItem(t.Context(), season.Items[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(filepath.Dir(source), "S01E04.mp4")
	if err := os.Rename(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	season, err = service.GetSeason(t.Context(), series.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	after, err := service.GetEpisodeItem(t.Context(), season.Items[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EpisodeNumbers[0] != 4 || after.Files[0].ID != before.Files[0].ID {
		t.Fatalf("rename = episodes %v file %d, want E4 and stable file %d",
			after.EpisodeNumbers, after.Files[0].ID, before.Files[0].ID)
	}
}

func TestScanReclassifiesTheSamePhysicalFileBothWays(t *testing.T) {
	service, root := newTestService(t)
	film := writeFile(t, root, "A Show Movie.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	episode := filepath.Join(root, "A.Show.S01E01.mkv")
	if err := os.Rename(film, episode); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	if count, _ := service.Count(t.Context()); count != 0 {
		t.Fatalf("film count after episode classification = %d, want 0", count)
	}
	if count, _ := service.SeriesCount(t.Context()); count != 1 {
		t.Fatalf("series count = %d, want 1", count)
	}

	filmAgain := filepath.Join(root, "A Show Movie.mkv")
	if err := os.Rename(episode, filmAgain); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	if count, _ := service.Count(t.Context()); count != 1 {
		t.Fatalf("film count after reverse classification = %d, want 1", count)
	}
	if count, _ := service.SeriesCount(t.Context()); count != 0 {
		t.Fatalf("series count after reverse classification = %d, want 0", count)
	}
}

func TestAmbiguousEpisodeIsReportedAndDoesNotBlockSafePruning(t *testing.T) {
	service, root := newTestService(t)
	known := writeFile(t, root, "Known Show/Known.Show.S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(known); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "S01E02.mkv")
	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Problems) != 1 || report.Problems[0].Kind != "episode_series_unknown" {
		t.Fatalf("problems = %+v", report.Problems)
	}
	if report.Removed != 1 {
		t.Fatalf("removed = %d, want the vanished known episode pruned", report.Removed)
	}
	if count, _ := service.SeriesCount(t.Context()); count != 0 {
		t.Fatalf("series count = %d, want 0", count)
	}
}

func TestSeriesInsideAShortsDirectoryIsNotHiddenByTheScanner(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Animation/Shorts/Bluey.S01E01.mkv")
	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.EpisodeFiles != 1 || report.Series != 1 {
		t.Fatalf("short episode scan = %+v", report)
	}
}

func TestSeriesAssociationDoesNotChooseBetweenConflictingTMDBIdentities(t *testing.T) {
	service, _ := newTestService(t)
	tx, err := service.store.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, tmdbID := range []int{101, 202} {
		if _, err := tx.ExecContext(t.Context(), `
			INSERT INTO series (title, year, tmdb_id, metadata_status, added_at, updated_at)
			VALUES ('Shared Name', 2020, ?, 'ok', 1, 1)`, tmdbID); err != nil {
			t.Fatal(err)
		}
	}
	first, err := ensureSeriesTx(t.Context(), tx, "Shared Name", 2020)
	if err != nil {
		t.Fatal(err)
	}
	if first <= 2 {
		t.Fatalf("association chose conflicting proven row %d", first)
	}
	second, err := ensureSeriesTx(t.Context(), tx, "Shared Name", 2020)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("unproven conflict bucket changed from %d to %d", first, second)
	}
}

func TestConsolidateSeriesUsesTMDBIdentityAndMergesFilesAndProgress(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Localized/Localized.S01E01.720p.mkv")
	writeFile(t, root, "Original/Original.S01E01.1080p.mp4")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	series, err := service.ListSeries(t.Context(), 10, 0)
	if err != nil || len(series) != 2 {
		t.Fatalf("series before consolidation = %+v, err = %v", series, err)
	}
	for index := range series {
		season, err := service.GetSeason(t.Context(), series[index].ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			if _, err := service.store.SaveEpisodeProgress(t.Context(), season.Items[0].ID,
				420, 1200, series[index].UpdatedAt.AddDate(0, 0, 1)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := service.store.db.ExecContext(t.Context(), `
		UPDATE series SET tmdb_id = 4242, metadata_status = 'ok', metadata_fetched_at = id`); err != nil {
		t.Fatal(err)
	}

	merged, err := service.store.ConsolidateSeries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if merged != 1 {
		t.Fatalf("merged = %d, want 1", merged)
	}
	canonical := onlySeries(t, service)
	season, err := service.GetSeason(t.Context(), canonical.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Items) != 1 {
		t.Fatalf("items after consolidation = %+v", season.Items)
	}
	item, err := service.GetEpisodeItem(t.Context(), season.Items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Files) != 2 || item.Progress.PositionSeconds != 420 {
		t.Fatalf("consolidated episode = %+v, want two files and progress 420", item)
	}
}

func onlySeries(t *testing.T, service *Service) Series {
	t.Helper()
	series, err := service.ListSeries(t.Context(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 {
		t.Fatalf("series count = %d, want one", len(series))
	}
	return series[0]
}
