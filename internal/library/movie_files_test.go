package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanGroupsParsedTitleAndYearIntoOneFilm(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "2001 A Space Odyssey 1968.mkv")
	writeFile(t, root, "2001_A_Space_Odyssey_1968_720p.mp4")

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Found != 2 || report.Added != 2 {
		t.Fatalf("report = %+v, want two files found and added", report)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 {
		t.Fatalf("listed %d films, want one grouped film", len(movies))
	}
	detail, err := service.Get(t.Context(), defaultProfileID, movies[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Files) != 2 {
		t.Fatalf("detail has %d files, want two", len(detail.Files))
	}
	if !detail.Files[0].IsPrimary || detail.Files[0].ID == detail.Files[1].ID {
		t.Errorf("file identities/primary are invalid: %+v", detail.Files)
	}
}

func TestScanKeepsRemakesWithDifferentYearsSeparate(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Suspiria (1977).mkv")
	writeFile(t, root, "Suspiria (2018).mkv")

	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 2 {
		t.Fatalf("listed %d films, want two remakes kept separate", len(movies))
	}
}

func TestDeletingPrimaryVariantKeepsFilmAndPromotesAnotherFile(t *testing.T) {
	service, root := newTestService(t)
	first := writeFile(t, root, "The Matrix 1999 1080p.mkv")
	second := writeFile(t, root, "The Matrix 1999 720p.mp4")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	before := onlyMovie(t, service)
	detail, err := service.Get(t.Context(), defaultProfileID, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	primaryPath := detail.Files[0].Path
	if primaryPath != first && primaryPath != second {
		t.Fatalf("unexpected primary path %q", primaryPath)
	}
	if err := os.Remove(primaryPath); err != nil {
		t.Fatal(err)
	}

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Errorf("removed = %d, want one file", report.Removed)
	}
	after := onlyMovie(t, service)
	if after.ID != before.ID {
		t.Errorf("film id changed from %d to %d", before.ID, after.ID)
	}
	detail, err = service.Get(t.Context(), defaultProfileID, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Files) != 1 || !detail.Files[0].IsPrimary {
		t.Fatalf("files after removal = %+v, want one promoted primary", detail.Files)
	}
	if detail.FileName != detail.Files[0].FileName || detail.Path != detail.Files[0].Path {
		t.Errorf("legacy primary fields were not mirrored")
	}
}

func TestMovePreservesMovieAndFileIDs(t *testing.T) {
	service, root := newTestService(t)
	original := writeFile(t, root, "Arrival (2016).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	before := onlyMovie(t, service)
	detail, err := service.Get(t.Context(), defaultProfileID, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	fileID := detail.Files[0].ID

	moved := filepath.Join(root, "Moved", "Arrival (2016).mkv")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Added != 0 || report.Removed != 0 || report.Updated != 1 {
		t.Errorf("move report = %+v, want one update and no add/remove", report)
	}
	after := onlyMovie(t, service)
	detail, err = service.Get(t.Context(), defaultProfileID, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || detail.Files[0].ID != fileID || detail.Files[0].Path != moved {
		t.Errorf("move changed identity: film %d->%d file %d->%d path %q",
			before.ID, after.ID, fileID, detail.Files[0].ID, detail.Files[0].Path)
	}
}

func TestRenamingSingleFileResetsStaleMetadataWithoutChangingIDs(t *testing.T) {
	service, root := newTestService(t)
	original := writeFile(t, root, "Arrival (2016).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	before := onlyMovie(t, service)
	detail, err := service.Get(t.Context(), defaultProfileID, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	fileID := detail.Files[0].ID
	if _, err := service.store.db.ExecContext(t.Context(), `
		UPDATE movies SET
			tmdb_id = 329865, tmdb_title = 'Arrival', overview = 'Old synopsis',
			release_date = '2016-11-10', poster_path = '/old.jpg',
			backdrop_path = '/old-backdrop.jpg', runtime_minutes = 116,
			vote_average = 7.6, director = 'Denis Villeneuve',
			genres_json = '["Drama"]', cast_json = '[{"name":"Amy Adams"}]',
			metadata_status = 'ok', metadata_fetched_at = 123
		WHERE id = ?`, before.ID); err != nil {
		t.Fatal(err)
	}

	renamed := filepath.Join(root, "Contact (1997).mkv")
	if err := os.Rename(original, renamed); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	after := onlyMovie(t, service)
	detail, err = service.Get(t.Context(), defaultProfileID, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || detail.Files[0].ID != fileID {
		t.Fatalf("rename changed film/file ids: %d/%d -> %d/%d",
			before.ID, fileID, after.ID, detail.Files[0].ID)
	}
	if after.Title != "Contact" || after.Year != 1997 {
		t.Errorf("renamed identity = %q (%d), want Contact (1997)", after.Title, after.Year)
	}
	metadata := after.Metadata
	if metadata.Status != statusPending || metadata.TMDBID != 0 || metadata.Title != "" ||
		metadata.Overview != "" || metadata.PosterPath != "" || metadata.Runtime != 0 ||
		len(metadata.Genres) != 0 || len(metadata.Cast) != 0 {
		t.Errorf("renamed film kept stale TMDB metadata: %+v", metadata)
	}
}

func TestConsolidateUsesTMDBIdentityAndPreservesProgress(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Blade Runner 2049.mkv")
	writeFile(t, root, "Blade_Runner_2049_2017_720p.mp4")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 2 {
		t.Fatalf("precondition: got %d films, want two before TMDB proves identity", len(movies))
	}
	wantID := min(movies[0].ID, movies[1].ID)
	for _, movie := range movies {
		status, poster := "ok", "/blade-runner.jpg"
		if movie.ID == wantID {
			status, poster = "not_found", ""
		}
		if _, err := service.store.db.ExecContext(t.Context(), `
			UPDATE movies SET tmdb_id = 335984, metadata_status = ?,
				tmdb_title = 'Blade Runner 2049', poster_path = ?, metadata_fetched_at = ?
			WHERE id = ?`, status, poster, time.Now().Unix(), movie.ID); err != nil {
			t.Fatal(err)
		}
	}
	newerID := movies[1].ID
	if _, err := service.store.SaveProgress(t.Context(), defaultProfileID, newerID, 1800, 9840, time.Now()); err != nil {
		t.Fatal(err)
	}

	merged, err := service.Consolidate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if merged != 1 {
		t.Fatalf("merged = %d, want one duplicate removed", merged)
	}
	after := onlyMovie(t, service)
	if after.ID != wantID {
		t.Errorf("surviving id = %d, want oldest %d", after.ID, wantID)
	}
	if after.Year != 2017 {
		t.Errorf("surviving parsed year = %d, want the known year 2017", after.Year)
	}
	if after.Progress.PositionSeconds != 1800 {
		t.Errorf("progress = %v, want 1800 from the most recently watched duplicate",
			after.Progress.PositionSeconds)
	}
	if after.Metadata.Status != statusOK || after.Metadata.PosterPath != "/blade-runner.jpg" {
		t.Errorf("metadata = %+v, want the recognised duplicate copied to the canonical id", after.Metadata)
	}
	detail, err := service.Get(t.Context(), defaultProfileID, after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Files) != 2 {
		t.Errorf("consolidated film has %d files, want two", len(detail.Files))
	}
}

func TestConsolidateRefusesConflictingTMDBIDs(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Dune (1984).mkv")
	writeFile(t, root, "Dune (2021).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, err := service.List(t.Context(), defaultProfileID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 2 {
		t.Fatal("precondition: remakes were already merged")
	}
	for i, movie := range movies {
		if _, err := service.store.db.ExecContext(t.Context(), `
			UPDATE movies SET title = 'Dune', year = 2021, tmdb_id = ?, metadata_status = 'ok'
			WHERE id = ?`, 100+i, movie.ID); err != nil {
			t.Fatal(err)
		}
	}
	merged, err := service.Consolidate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if merged != 0 {
		t.Errorf("merged = %d, want conflicting TMDB identities kept separate", merged)
	}
	if count, _ := service.Count(t.Context()); count != 2 {
		t.Errorf("count = %d, want two separate films", count)
	}
}

func TestMediaInspectionKeepsTrackIDsAndInvalidatesWhenFileChanges(t *testing.T) {
	service, root := newTestService(t)
	path := writeFile(t, root, "Heat (1995).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movie := onlyMovie(t, service)
	detail, _ := service.Get(t.Context(), defaultProfileID, movie.ID)
	fileID := detail.Files[0].ID

	media := FileMedia{
		Status:          MediaOK,
		Container:       "matroska,webm",
		DurationSeconds: 10200,
		Video:           &VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []AudioTrack{
			{StreamIndex: 1, Codec: "dts", Language: "eng", Channels: "5.1", IsDefault: true},
			{StreamIndex: 2, Codec: "ac3", Language: "fre", Channels: "5.1"},
		},
	}
	first, err := service.SaveFileMedia(t.Context(), movie.ID, fileID, media)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Media.AudioTracks) != 2 || first.Media.AudioTracks[0].ID == 0 {
		t.Fatalf("saved tracks = %+v, want stable database ids", first.Media.AudioTracks)
	}
	firstID := first.Media.AudioTracks[0].ID
	media.AudioTracks = media.AudioTracks[:1]
	media.AudioTracks[0].Title = "Original mix"
	second, err := service.SaveFileMedia(t.Context(), movie.ID, fileID, media)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Media.AudioTracks) != 1 || second.Media.AudioTracks[0].ID != firstID {
		t.Errorf("reinspection changed track identity: before %d after %+v", firstID, second.Media.AudioTracks)
	}

	// Change both size and mtime so the scanner can prove the cached probe is
	// stale. A scan must clear codecs and track ids before exposing the file.
	if err := os.WriteFile(path, []byte("replacement media bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	after, err := service.GetMovieFile(t.Context(), movie.ID, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Media.Status != MediaPending || after.Media.Video != nil || len(after.Media.AudioTracks) != 0 {
		t.Errorf("changed file kept stale media: %+v", after.Media)
	}
}

func TestAudioTrackCannotCrossFileOwnership(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Alien (1979).mkv")
	writeFile(t, root, "Aliens (1986).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	movies, _ := service.List(t.Context(), defaultProfileID, 10, 0)
	first, _ := service.Get(t.Context(), defaultProfileID, movies[0].ID)
	second, _ := service.Get(t.Context(), defaultProfileID, movies[1].ID)
	inspected, err := service.SaveFileMedia(t.Context(), first.ID, first.Files[0].ID, FileMedia{
		Status: MediaOK,
		Video:  &VideoStream{StreamIndex: 0, Codec: "h264"},
		AudioTracks: []AudioTrack{
			{StreamIndex: 1, Codec: "aac"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	trackID := inspected.Media.AudioTracks[0].ID
	_, err = service.AudioTrack(t.Context(), second.ID, second.Files[0].ID, trackID)
	if !errors.Is(err, ErrNoSuchAudioTrack) {
		t.Errorf("cross-file track lookup error = %v, want ErrNoSuchAudioTrack", err)
	}
}
