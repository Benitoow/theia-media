package library

import "testing"

func TestSearchKeyFoldsWhatAKeyboardCannotEasilyType(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Amélie", "amelie"},
		{"LE FABULEUX DESTIN", "le fabuleux destin"},
		{"Cœur fidèle", "coeur fidele"},
		{"WALL·E", "wall e"},
		{"Das Weiße Band", "das weisse band"},
		{"Á bout de souffle", "a bout de souffle"},
		{"  spaced   out  ", "spaced out"},
		{"Se7en", "se7en"},
		{"", ""},
		{"...", ""},
	}
	for _, tt := range tests {
		if got := searchKey(tt.in); got != tt.want {
			t.Errorf("searchKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSearchFindsAnAccentedTitleTypedWithout(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Amélie (2001).mkv")
	writeFile(t, root, "Heat (1995).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	results, err := service.Search(t.Context(), defaultProfileID, "amelie")
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Movies) != 1 {
		t.Fatalf("matched %d films, want 1", len(results.Movies))
	}
	if results.Movies[0].Title != "Amélie" {
		t.Errorf("matched %q, want Amélie", results.Movies[0].Title)
	}
}

func TestSearchIsAboutTitlesNotFileNames(t *testing.T) {
	// The release group, the resolution and the codec are in every filename and
	// in none of them does anybody mean to search for them.
	service, root := newTestService(t)
	writeFile(t, root, "The.Matrix.1999.1080p.BluRay.x264-RARBG.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	for _, needle := range []string{"1080p", "rarbg", "x264"} {
		results, err := service.Search(t.Context(), defaultProfileID, needle)
		if err != nil {
			t.Fatal(err)
		}
		if len(results.Movies) != 0 {
			t.Errorf("searching %q matched %d films, want none", needle, len(results.Movies))
		}
	}

	results, err := service.Search(t.Context(), defaultProfileID, "matrix")
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Movies) != 1 {
		t.Errorf("searching the title matched %d films, want 1", len(results.Movies))
	}
}

func TestSearchMatchesTheYear(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Heat (1995).mkv")
	writeFile(t, root, "Se7en (1995).mkv")
	writeFile(t, root, "Fargo (1996).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	results, err := service.Search(t.Context(), defaultProfileID, "1995")
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Movies) != 2 {
		t.Errorf("matched %d films for 1995, want 2", len(results.Movies))
	}
}

func TestSearchSpansBothCatalogues(t *testing.T) {
	// The point of the whole thing: not having to know whether what you are
	// looking for is a film or a series before you are allowed to look.
	service, root := newTestService(t)
	writeFile(t, root, "Twin Peaks (1992).mkv")
	writeFile(t, root, "Twin Peaks/Season 1/Twin Peaks S01E01.mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	results, err := service.Search(t.Context(), defaultProfileID, "twin peaks")
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Movies) != 1 {
		t.Errorf("matched %d films, want 1", len(results.Movies))
	}
	if len(results.Series) != 1 {
		t.Errorf("matched %d series, want 1", len(results.Series))
	}
}

func TestAnEmptySearchIsNotEveryFilm(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Heat (1995).mkv")
	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	for _, needle := range []string{"", "   ", "..."} {
		results, err := service.Search(t.Context(), defaultProfileID, needle)
		if err != nil {
			t.Fatal(err)
		}
		if len(results.Movies) != 0 || len(results.Series) != 0 {
			t.Errorf("searching %q returned %d films and %d series, want nothing",
				needle, len(results.Movies), len(results.Series))
		}
	}
}
