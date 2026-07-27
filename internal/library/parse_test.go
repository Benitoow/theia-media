package library

import "testing"

func TestParseFileName(t *testing.T) {
	// maxYear is pinned so the table does not start failing on 1 January.
	const maxYear = 2027

	tests := []struct {
		name      string
		in        string
		wantTitle string
		wantYear  int
	}{
		// --- the common cases -------------------------------------------------
		{
			name:      "scene naming with dots",
			in:        "The.Matrix.1999.1080p.BluRay.x264.mkv",
			wantTitle: "The Matrix",
			wantYear:  1999,
		},
		{
			name:      "year in parentheses",
			in:        "Blade Runner 2049 (2017) [1080p].mkv",
			wantTitle: "Blade Runner 2049",
			wantYear:  2017,
		},
		{
			name:      "plain title and year",
			in:        "Le Salaire de la peur (1953).mkv",
			wantTitle: "Le Salaire de la peur",
			wantYear:  1953,
		},
		{
			name:      "underscores as separators",
			in:        "Mad_Max_Fury_Road_2015_720p.mp4",
			wantTitle: "Mad Max Fury Road",
			wantYear:  2015,
		},
		{
			name:      "release group suffix",
			in:        "Dune.Part.Two.2024.2160p.WEB-DL.DDP5.1.Atmos.HDR.H265-FLUX.mkv",
			wantTitle: "Dune Part Two",
			wantYear:  2024,
		},

		// --- numbers that are not years --------------------------------------
		{
			name: "a title ending in a future year keeps it",
			// 2049 is beyond any plausible release date, so it belongs to the
			// title. Getting this wrong yields a film called "Blade Runner"
			// released in 2049.
			in:        "Blade Runner 2049.mkv",
			wantTitle: "Blade Runner 2049",
			wantYear:  0,
		},
		{
			name: "title number first, release year last",
			// Both are plausible years; the last one is the release date.
			in:        "2001 A Space Odyssey 1968.mkv",
			wantTitle: "2001 A Space Odyssey",
			wantYear:  1968,
		},
		{
			name:      "numeric title with a release year",
			in:        "1917.2019.1080p.BluRay.mkv",
			wantTitle: "1917",
			wantYear:  2019,
		},
		{
			name: "numeric title with no release year",
			// The year is all there is, so it has to be the title -- a row with
			// an empty name and a year of 1917 helps nobody.
			in:        "1917.mkv",
			wantTitle: "1917",
			wantYear:  0,
		},
		{
			name:      "a year-like title with its own release year",
			in:        "2012 (2009).mkv",
			wantTitle: "2012",
			wantYear:  2009,
		},
		{
			name:      "resolution is not mistaken for a year",
			in:        "Interstellar.2014.2160p.mkv",
			wantTitle: "Interstellar",
			wantYear:  2014,
		},

		// --- degrading gracefully --------------------------------------------
		{
			name:      "no year at all",
			in:        "Amelie.mkv",
			wantTitle: "Amelie",
			wantYear:  0,
		},
		{
			name:      "no year but plenty of tags",
			in:        "Spirited.Away.1080p.BluRay.x264-GROUP.mkv",
			wantTitle: "Spirited Away",
			wantYear:  0,
		},
		{
			name: "leading release group in brackets",
			// Cutting at the first tag would leave nothing if the group came
			// first, so bracketed groups are removed before anything else.
			in:        "[YTS.MX] The Godfather (1972) [1080p].mp4",
			wantTitle: "The Godfather",
			wantYear:  1972,
		},
		{
			name:      "a title containing a word that is also a tag",
			in:        "The Final Cut (2004).mkv",
			wantTitle: "The Final Cut",
			wantYear:  2004,
		},
		{
			name:      "hyphens inside a title survive",
			in:        "Spider-Man.Into.the.Spider-Verse.2018.mkv",
			wantTitle: "Spider-Man Into the Spider-Verse",
			wantYear:  2018,
		},
		{
			name:      "accented French title",
			in:        "Les Quatre Cents Coups (1959).mkv",
			wantTitle: "Les Quatre Cents Coups",
			wantYear:  1959,
		},

		// --- outright malformed, must not panic or return empty ---------------
		{
			name:      "nothing but an extension",
			in:        ".mkv",
			wantTitle: ".mkv",
			wantYear:  0,
		},
		{
			name:      "empty string",
			in:        "",
			wantTitle: "",
			wantYear:  0,
		},
		{
			name: "only punctuation",
			// Separator handling turns this into nothing, so the fallback takes
			// over. Useless as a title, but it is the filename the user can go
			// and look at, which is the point of degrading rather than failing.
			in:        "___.mkv",
			wantTitle: "___",
			wantYear:  0,
		},
		{
			name:      "only tags, no title",
			in:        "1080p.BluRay.x264.mkv",
			wantTitle: "1080p BluRay x264",
			wantYear:  0,
		},
		{
			name:      "no extension at all",
			in:        "The Thing 1982",
			wantTitle: "The Thing",
			wantYear:  1982,
		},
		{
			name:      "a numeric suffix is a year, not an extension",
			in:        "Alien.1979",
			wantTitle: "Alien",
			wantYear:  1979,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFileName(tt.in, maxYear)
			if got.Title != tt.wantTitle || got.Year != tt.wantYear {
				t.Errorf("parseFileName(%q)\n got  title=%q year=%d\n want title=%q year=%d",
					tt.in, got.Title, got.Year, tt.wantTitle, tt.wantYear)
			}
		})
	}
}

// The scanner writes whatever comes back straight into a NOT NULL column, so
// an empty title is a constraint violation waiting for the one badly named file
// in a real library.
func TestParseFileNameNeverReturnsAnEmptyTitleForRealInput(t *testing.T) {
	inputs := []string{
		"a.mkv", "  .mkv", "....mkv", "[].mkv", "()mkv", "-.mkv", "1080p.mkv",
		"...", "()[]{}", "é.mkv", "2049.mkv", "0000.mkv", "映画.mkv",
	}
	for _, in := range inputs {
		if got := parseFileName(in, 2027); got.Title == "" {
			t.Errorf("parseFileName(%q) returned an empty title", in)
		}
	}
}

func TestParseFileNameUsesTheCurrentYearAsItsUpperBound(t *testing.T) {
	// ParseFileName, unlike parseFileName, derives its bound from the clock.
	// A film dated next year is plausible; one dated in 2099 is a title.
	if got := ParseFileName("Some Film 2099.mkv"); got.Year != 0 {
		t.Errorf("year = %d, want 0: 2099 is not a plausible release year", got.Year)
	}
	if got := ParseFileName("Some Film 1994.mkv"); got.Year != 1994 {
		t.Errorf("year = %d, want 1994", got.Year)
	}
}
