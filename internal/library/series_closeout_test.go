package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Generated from a real scanned store, shared with the native decoder.
func TestSeriesCloseoutContract(t *testing.T) {
	service, root := newTestService(t)
	for _, file := range []string{"Show/Show.S00E01.mkv", "Show/Show.S01E01.mkv", "Show/Show.S01E02.mkv", "Show/Show.S02E01.mkv"} {
		writeFile(t, root, file)
	}
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	show := onlySeries(t, service)
	if _, err := service.store.db.ExecContext(t.Context(), `UPDATE series SET original_language='en',tmdb_name='The Show',metadata_status='ok' WHERE id=?`, show.ID); err != nil {
		t.Fatal(err)
	}
	first, err := service.GetSeason(t.Context(), defaultProfileID, show.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.GetSeason(t.Context(), defaultProfileID, show.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 20, 0, 0, 0, time.UTC)
	for index, item := range append(first.Items, second.Items...) {
		if _, err := service.store.SaveEpisodeProgress(t.Context(), defaultProfileID, item.ID, 420, 1200, now.Add(time.Duration(index)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	continued, err := service.store.ContinueEpisodes(t.Context(), defaultProfileID, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(continued) != 1 || continued[0].ID != second.Items[0].ID || len(continued[0].Episodes) != 1 || len(continued[0].Files) != 0 {
		t.Fatalf("grouped continue row: %+v", continued)
	}
	// Equal timestamps have a deterministic id tie-break, and completion clears the choice.
	for _, item := range first.Items {
		if _, err := service.store.SaveEpisodeProgress(t.Context(), defaultProfileID, item.ID, 420, 1200, now.Add(10*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	continued, err = service.store.ContinueEpisodes(t.Context(), defaultProfileID, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(continued) != 1 || continued[0].ID != first.Items[1].ID {
		t.Fatalf("timestamp tie: %+v", continued)
	}
	if _, err := service.store.SetEpisodeWatched(t.Context(), defaultProfileID, first.Items[0].ID, now); err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetSeries(t.Context(), defaultProfileID, show.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.NextUnwatched == nil || detail.NextUnwatched.ID != first.Items[1].ID || detail.NextUnwatched.OriginalLanguage != "en" {
		t.Fatalf("next unwatched: %+v", detail.NextUnwatched)
	}
	if detail.ResumeEpisode == nil || detail.ResumeEpisode.ID != first.Items[1].ID {
		t.Fatalf("resume: %+v", detail.ResumeEpisode)
	}
	if err := service.ResetEpisodeProgress(t.Context(), defaultProfileID, first.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	detail, err = service.GetSeries(t.Context(), defaultProfileID, show.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.NextUnwatched == nil || detail.NextUnwatched.ID != first.Items[0].ID {
		t.Fatal("reset did not restore the starting point")
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	normaliseContract(value)
	raw, err = json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("..", "..", "player", "contract-fixtures", "series-detail.json")
	if os.Getenv("THEIA_WRITE_CONTRACT") == "1" {
		if err := os.MkdirAll(filepath.Dir(fixture), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture, append(raw, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var golden any
	if err := json.Unmarshal(expected, &golden); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, golden) {
		t.Fatalf("Go JSON contract changed; review the fixture and native decoder\n%s", raw)
	}
}

func normaliseContract(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if strings.HasSuffix(key, "_at") {
				if _, ok := item.(string); ok {
					v[key] = "2026-09-29T20:00:00Z"
					continue
				}
			}
			normaliseContract(item)
		}
	case []any:
		for _, item := range v {
			normaliseContract(item)
		}
	}
}
