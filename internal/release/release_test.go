package release

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubRelease serves a release the way GitHub does: a JSON document naming the
// assets and their digests, and the bytes at the addresses it gives.
//
// The digests are computed from the bytes actually served, so a test that
// corrupts a payload is testing the check rather than the fixture.
func stubRelease(t *testing.T, tag string, payloads map[string][]byte) (*httptest.Server, Release) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/n/releases/latest" {
			rel := Release{Tag: tag, URL: server.URL + "/tag/" + tag}
			for name, body := range payloads {
				sum := sha256.Sum256(body)
				rel.Assets = append(rel.Assets, Asset{
					Name:   name,
					Size:   int64(len(body)),
					URL:    server.URL + "/assets/" + name,
					Digest: "sha256:" + hex.EncodeToString(sum[:]),
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rel)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/assets/")
		body, ok := payloads[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)

	ctx := context.Background()
	rel, err := Latest(ctx, server.Client(), server.URL, "o/n")
	if err != nil {
		t.Fatalf("Latest against the stub: %v", err)
	}
	return server, rel
}

func TestTheNamesMatchWhatTheReleaseWorkflowPublishes(t *testing.T) {
	// These strings are a contract with .github/workflows/release.yml and with
	// the updater inside every installed v3.2. Changing one here without
	// changing it there is an installation that can never find its own release.
	cases := map[string]string{
		ServerName("windows", "amd64"):           "theia-server-windows-amd64.exe",
		ServerName("linux", "arm64"):             "theia-server-linux-arm64",
		SetupName("windows", "amd64"):            "theia-setup-windows-amd64.exe",
		SetupName("darwin", "arm64"):             "theia-setup-darwin-arm64",
		PlayerName("windows", "amd64"):           "theia-player-windows-amd64.zip",
		LauncherName("windows", "amd64"):         "theia-launcher-windows-amd64.exe",
		ArchiveName("3.3.0", "windows", "amd64"): "theia-3.3.0-windows-amd64.zip",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("asset name = %q, want %q", got, want)
		}
	}
}

func TestLatestReportsARepositoryWithoutReleases(t *testing.T) {
	// A young project with no release yet is an ordinary state, not a crash,
	// and the installer has to be able to say so without alarming anybody.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	if _, err := Latest(context.Background(), server.Client(), server.URL, "o/n"); err != ErrNoRelease {
		t.Errorf("Latest = %v, want ErrNoRelease", err)
	}
}

func TestDownloadRefusesAnAssetWithNoDigest(t *testing.T) {
	// The rule the whole project rests on: an unverified binary never reaches a
	// disk. An asset the release page describes without a digest is refused
	// before a single byte is requested.
	server, rel := stubRelease(t, "v9.9.9", map[string][]byte{"theia-server-windows-amd64.exe": []byte("MZ fake")})
	asset, err := rel.Named("theia-server-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}
	asset.Digest = ""

	if _, err := asset.Download(context.Background(), server.Client(), t.TempDir(), nil); err == nil {
		t.Fatal("a download without a digest was accepted")
	} else if !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

func TestDownloadVerifiesTheDigestAndOnlyThenNamesTheFile(t *testing.T) {
	payload := bytes.Repeat([]byte("theia"), 5000)
	server, rel := stubRelease(t, "v9.9.9", map[string][]byte{"theia-server-windows-amd64.exe": payload})
	asset, err := rel.Named("theia-server-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	var seen int64
	path, err := asset.Download(context.Background(), server.Client(), dir, func(done, total int64) {
		seen = done
		if total != int64(len(payload)) {
			t.Errorf("progress total = %d, want %d", total, len(payload))
		}
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if seen != int64(len(payload)) {
		t.Errorf("progress stopped at %d of %d bytes", seen, len(payload))
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, payload) {
		t.Error("the file on disk is not the file that was served")
	}
	if filepath.Base(path) != "theia-server-windows-amd64.exe" {
		t.Errorf("the file was named %q", filepath.Base(path))
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(leftovers) != 0 {
		t.Errorf("a partial file was left behind: %v", leftovers)
	}

	// And a payload that does not match the advertised digest leaves nothing at
	// all behind - not under the real name, and not under the partial one.
	rel.Assets[0].Digest = "sha256:" + strings.Repeat("0", 64)
	bad, err := rel.Named("theia-server-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if _, err := bad.Download(context.Background(), server.Client(), other, nil); err == nil {
		t.Fatal("a corrupted download was accepted")
	} else if !strings.Contains(err.Error(), "expected") {
		t.Errorf("the refusal does not name both digests: %v", err)
	}
	entries, err := os.ReadDir(other)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Errorf("a refused download left %s behind", entry.Name())
	}
}

func TestDownloadRefusesAShortFile(t *testing.T) {
	// A truncated transfer that somehow hashed correctly would still be a
	// broken program; the announced size is the second check.
	payload := []byte("MZ this is a program, honestly")
	server, rel := stubRelease(t, "v9.9.9", map[string][]byte{"theia-server.exe": payload})
	asset, err := rel.Named("theia-server.exe")
	if err != nil {
		t.Fatal(err)
	}
	asset.Size = int64(len(payload)) + 4096

	if _, err := asset.Download(context.Background(), server.Client(), t.TempDir(), nil); err == nil {
		t.Fatal("a file shorter than the release announced was accepted")
	}
}

func TestExtractTakesOnlyWhatWasAskedFor(t *testing.T) {
	archive := makeArchive(t, map[string]string{
		"theia-player.exe":   "player",
		"libmpv-2.dll":       "engine",
		"LICENSE-libmpv.txt": "lgpl",
		"NOTICE.md":          "notice",
		"something-else.dll": "not ours",
	})
	dir := t.TempDir()
	written, err := Extract(archive, dir, []string{"theia-player.exe", "libmpv-2.dll", "LICENSE-libmpv.txt", "NOTICE.md"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(written) != 4 {
		t.Errorf("extracted %v, want four files", written)
	}
	for _, name := range []string{"theia-player.exe", "libmpv-2.dll", "LICENSE-libmpv.txt", "NOTICE.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s was not extracted: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "something-else.dll")); err == nil {
		t.Error("the extraction took a file nobody asked for")
	}
}

func TestExtractRefusesAnIncompleteBundle(t *testing.T) {
	// A player bundle without its licence is a licence breach. Half an
	// extraction is a failure, and it says which file was missing.
	archive := makeArchive(t, map[string]string{"theia-player.exe": "player"})
	_, err := Extract(archive, t.TempDir(), []string{"theia-player.exe", "LICENSE-libmpv.txt"})
	if err == nil {
		t.Fatal("a bundle missing its licence was accepted")
	}
	if !strings.Contains(err.Error(), "LICENSE-libmpv.txt") {
		t.Errorf("the refusal does not name the missing file: %v", err)
	}
}

func TestExtractRefusesAnEntryOutsideTheDestination(t *testing.T) {
	// The archive comes from somewhere else. An entry named ../../evil is not
	// unpacked over whatever it reaches.
	archive := makeArchive(t, map[string]string{"../escaped.exe": "nope", "theia-player.exe": "player"})
	dir := t.TempDir()
	_, err := Extract(archive, filepath.Join(dir, "inside"), []string{"../escaped.exe"})
	if err == nil {
		t.Fatal("an entry climbing out of the destination was extracted")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.exe")); err == nil {
		t.Error("the file was written outside the destination after all")
	}
}

func makeArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
