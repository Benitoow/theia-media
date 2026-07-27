package scanner

import "testing"

func TestLooksLikeSample(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"a sample file", "The.Matrix.1999.sample.mkv", true},
		{"a trailer", "Inception.2010.trailer.mkv", true},
		{"sample on its own", "sample.mkv", true},
		{"bracketed", "Movie (2010) [sample].mkv", true},

		// The regression this file exists for. "rarbg" was once in the word
		// list and quietly deleted every film released by that group -- nine
		// of them out of 277 in the test library, all of them real films.
		// Release group names describe who packaged the file, not what it is.
		{"release group in the suffix", "Chinatown.1974.1080p.BluRay.x264-RARBG.mkv", false},
		{"release group in brackets", "[RARBG] Apocalypse Now (1979) [1080p].mp4", false},
		{"another group", "Dune.2021.2160p.WEB-DL-FLUX.mkv", false},

		// "sample" inside a longer word is part of a title, not a marker.
		{"substring is not a match", "The Sampler (1998).mkv", false},
		{"ordinary film", "Alien (1979).mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeSample(tt.in); got != tt.want {
				t.Errorf("looksLikeSample(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSkippedDirectoriesCoverTheUsualSuspects(t *testing.T) {
	for _, name := range []string{"extras", "Extras", "EXTRAS", "Featurettes", "Sample", "Trailers", "@eaDir"} {
		if !skippedDirectories[lower(name)] {
			t.Errorf("directory %q should be skipped", name)
		}
	}
	for _, name := range []string{"Films", "Cinema", "Action", "1990s"} {
		if skippedDirectories[lower(name)] {
			t.Errorf("directory %q should not be skipped", name)
		}
	}
}

// lower mirrors what scanRoot does before the lookup.
func lower(s string) string {
	b := []rune(s)
	for i, r := range b {
		if r >= 'A' && r <= 'Z' {
			b[i] = r + 32
		}
	}
	return string(b)
}
