package library

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestEpisodeMediaKeepsTrackIDsAndInvalidatesWhenFileChanges(t *testing.T) {
	service, root := newTestService(t)
	path := writeFile(t, root, "Show/Show.S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	series := onlySeries(t, service)
	season, _ := service.GetSeason(t.Context(), series.ID, 1)
	item, _ := service.GetEpisodeItem(t.Context(), season.Items[0].ID)
	file := item.Files[0]

	measured := FileMedia{
		Status:          MediaOK,
		Container:       "matroska",
		DurationSeconds: 1500,
		Video:           &VideoStream{StreamIndex: 0, Codec: "h264", Width: 1920, Height: 1080},
		AudioTracks: []AudioTrack{
			{StreamIndex: 1, Codec: "aac", Language: "eng", IsDefault: true},
			{StreamIndex: 2, Codec: "aac", Language: "fra"},
		},
	}
	first, err := service.SaveEpisodeFileMedia(t.Context(), item.ID, file.ID, measured)
	if err != nil {
		t.Fatal(err)
	}
	firstIDs := []int64{first.Media.AudioTracks[0].ID, first.Media.AudioTracks[1].ID}
	measured.AudioTracks = []AudioTrack{
		{StreamIndex: 1, Codec: "aac", Language: "en", IsDefault: true},
		{StreamIndex: 3, Codec: "ac3", Language: "fra"},
	}
	second, err := service.SaveEpisodeFileMedia(t.Context(), item.ID, file.ID, measured)
	if err != nil {
		t.Fatal(err)
	}
	if second.Media.AudioTracks[0].ID != firstIDs[0] {
		t.Errorf("stable stream 1 id changed from %d to %d", firstIDs[0], second.Media.AudioTracks[0].ID)
	}
	if second.Media.AudioTracks[1].ID == firstIDs[1] {
		t.Errorf("new stream 3 reused removed stream 2 id %d", firstIDs[1])
	}
	if _, err := service.EpisodeAudioTrack(t.Context(), item.ID, file.ID, firstIDs[1]); !errors.Is(err, ErrNoSuchAudioTrack) {
		t.Errorf("removed track lookup err = %v", err)
	}

	if err := os.WriteFile(path, []byte(strings.Repeat("changed", 300)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	item, err = service.GetEpisodeItem(t.Context(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Files[0].Media.Status != MediaPending || len(item.Files[0].Media.AudioTracks) != 0 ||
		item.Files[0].Media.Video != nil {
		t.Fatalf("changed file media was not invalidated: %+v", item.Files[0].Media)
	}
}

func TestEpisodeProgressUsesTheSameResumeAndFinishedRulesAsFilms(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Show/Show.S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	series := onlySeries(t, service)
	season, _ := service.GetSeason(t.Context(), series.ID, 1)
	id := season.Items[0].ID

	short, err := service.SaveEpisodeProgress(t.Context(), id, 10, 1200)
	if err != nil {
		t.Fatal(err)
	}
	if short.PositionSeconds != 0 || short.WatchedAt != nil {
		t.Fatalf("short opening was remembered: %+v", short)
	}
	finished, err := service.SaveEpisodeProgress(t.Context(), id, 1190, 1200)
	if err != nil {
		t.Fatal(err)
	}
	if !finished.Finished {
		t.Fatalf("near-end episode was not finished: %+v", finished)
	}
	if err := service.ResetEpisodeProgress(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	item, err := service.GetEpisodeItem(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Progress.PositionSeconds != 0 || item.Progress.Finished || item.Progress.DurationSeconds != 1200 {
		t.Fatalf("reset progress = %+v", item.Progress)
	}
}
