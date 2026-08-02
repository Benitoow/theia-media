package library

import (
	"reflect"
	"testing"
)

func TestParseEpisodePathRecognisesSupportedPatterns(t *testing.T) {
	tests := []struct {
		path      string
		series    string
		year      int
		season    int
		episodes  []int
		title     string
		ambiguous bool
	}{
		{"The.Bear.S02E03.Sundae.mkv", "The Bear", 0, 2, []int{3}, "Sundae", false},
		{"The.Office.2005.S01E02.mkv", "The Office", 2005, 1, []int{2}, "", false},
		{"Show.Name.S01E01E02.1080p.mkv", "Show Name", 0, 1, []int{1, 2}, "", false},
		{"Show.Name.S01E01-E02.mkv", "Show Name", 0, 1, []int{1, 2}, "", false},
		{"Show Name 1x04 Episode Title.mp4", "Show Name", 0, 1, []int{4}, "Episode Title", false},
		{"The Expanse/Season 02/S02E06 - Immolation.mkv", "The Expanse", 0, 2, []int{6}, "Immolation", false},
		{"Doctor Who/Specials/Doctor.Who.S00E01.mkv", "Doctor Who", 0, 0, []int{1}, "", false},
		{"S01E02.mkv", "", 0, 1, []int{2}, "", true},
	}

	for _, tt := range tests {
		got := ParseEpisodePath(tt.path)
		if !got.Matched || got.SeriesTitle != tt.series || got.SeriesYear != tt.year ||
			got.SeasonNumber != tt.season || !reflect.DeepEqual(got.EpisodeNumbers, tt.episodes) ||
			got.EpisodeTitle != tt.title || got.Ambiguous != tt.ambiguous {
			t.Errorf("ParseEpisodePath(%q) = %+v", tt.path, got)
		}
	}
}

func TestParseEpisodePathRejectsFilmsAndMalformedMarkers(t *testing.T) {
	for _, input := range []string{
		"Se7en.1995.mkv", "1917.2019.mkv", "Room.104.2017.mkv",
		"2026-08-02.mkv", "ProjectS01E02.mkv", "Show.S01E02X.mkv",
		"Show.S001E01.mkv", "Show.S01E1000.mkv", "Show.101.mkv",
		"Show.Season.1.Episode.2.mkv", "SxxEyy.mkv",
	} {
		if got := ParseEpisodePath(input); got.Matched {
			t.Errorf("ParseEpisodePath(%q) matched unexpectedly: %+v", input, got)
		}
	}
}
