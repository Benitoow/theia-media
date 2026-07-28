package updater

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current   string
		newer, comparable bool
	}{
		{"v1.1.0", "1.0.0", true, true},
		{"1.0.1", "1.0.0", true, true},
		{"v2.0.0", "v1.9.9", true, true},
		{"v1.0.0", "1.0.0", false, true},
		{"v1.0.0", "1.1.0", false, true},
		// Ten sorts after nine, which string comparison gets wrong.
		{"v1.10.0", "v1.9.0", true, true},
		{"v1.0.10", "v1.0.9", true, true},

		// A release outranks any prerelease of the same number.
		{"v1.0.0", "v1.0.0-rc.1", true, true},
		{"v1.0.0-rc.1", "v1.0.0", false, true},
		{"v1.0.0-rc.2", "v1.0.0-rc.1", true, true},
		{"v1.0.0-beta", "v1.0.0-alpha", true, true},

		// Anything unparseable must refuse to compare, not guess. A build that
		// cannot say what it is has no business replacing itself.
		{"v1.1.0", "dev", false, false},
		{"v1.1.0", "a1b2c3d", false, false},
		{"v1.1.0", "", false, false},
		{"main", "1.0.0", false, false},
		{"", "1.0.0", false, false},
	}

	for _, tt := range tests {
		newer, comparable := isNewer(tt.latest, tt.current)
		if newer != tt.newer || comparable != tt.comparable {
			t.Errorf("isNewer(%q, %q) = (%v, %v), want (%v, %v)",
				tt.latest, tt.current, newer, comparable, tt.newer, tt.comparable)
		}
	}
}

func TestParseVersion(t *testing.T) {
	for _, s := range []string{"1.2.3", "v1.2.3", "v0.0.1", "v1.0.0-rc.1", "10.20.30"} {
		if _, ok := parseVersion(s); !ok {
			t.Errorf("parseVersion(%q) failed, want success", s)
		}
	}
	for _, s := range []string{"dev", "", "1.2", "v1", "latest", "a1b2c3d", "1.2.3.4"} {
		if _, ok := parseVersion(s); ok {
			t.Errorf("parseVersion(%q) succeeded, want failure", s)
		}
	}
}

func TestAssetName(t *testing.T) {
	// This has to match the release workflow exactly. A mismatch produces an
	// update that can never find its own binary.
	tests := map[string]string{
		"windows/amd64": "theia-windows-amd64.exe",
		"windows/arm64": "theia-windows-arm64.exe",
		"linux/amd64":   "theia-linux-amd64",
		"darwin/arm64":  "theia-darwin-arm64",
	}
	for platform, want := range tests {
		var goos, goarch string
		for i := range platform {
			if platform[i] == '/' {
				goos, goarch = platform[:i], platform[i+1:]
				break
			}
		}
		if got := assetName(goos, goarch); got != want {
			t.Errorf("assetName(%s) = %q, want %q", platform, got, want)
		}
	}
}

func TestAssetForRejectsAnUnverifiableRelease(t *testing.T) {
	rel := &release{
		TagName: "v1.1.0",
		Assets: []asset{
			{Name: "theia-linux-amd64", Digest: ""},
			{Name: "theia-linux-arm64", Digest: "sha256:notlongenough"},
			{Name: "theia-darwin-arm64", Digest: "md5:" + string(make([]byte, 64))},
		},
	}

	for _, platform := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "arm64"}} {
		if _, _, err := assetFor(rel, platform[0], platform[1]); err == nil {
			t.Errorf("assetFor(%s/%s) accepted an asset with no usable digest", platform[0], platform[1])
		}
	}

	// And one that is fine.
	good := &release{TagName: "v1.1.0", Assets: []asset{{
		Name:   "theia-linux-amd64",
		Digest: "sha256:e7e7fb30477f717e6f55f9180a70386c62677ef8a4d4d1a5d948f4098aa3eb99",
	}}}
	_, digest, err := assetFor(good, "linux", "amd64")
	if err != nil {
		t.Fatalf("assetFor rejected a well-formed asset: %v", err)
	}
	if len(digest) != 64 {
		t.Errorf("digest = %q, want the bare 64-character hex", digest)
	}
}

func TestAssetForReportsAMissingPlatform(t *testing.T) {
	rel := &release{TagName: "v1.1.0", Assets: []asset{{Name: "theia-linux-amd64"}}}
	if _, _, err := assetFor(rel, "openbsd", "riscv64"); err == nil {
		t.Error("assetFor found a binary for a platform the release does not ship")
	}
}
