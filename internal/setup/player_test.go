package setup

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
	"runtime"
	"testing"

	"github.com/Benitoow/theia-media/internal/release"
)

// writeBundle puts a bundle's files on disk with recognisable content, so a
// replacement can be told from what was already there.
func writeBundle(t *testing.T, dir, marker string) map[string]string {
	t.Helper()
	before := map[string]string{}
	for _, name := range bundleFiles(runtime.GOOS) {
		body := marker + " " + name
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		before[name] = digestOf([]byte(body))
	}
	return before
}

// assertUnchanged is the promise every refusal makes: the installation is
// exactly as it was.
func assertUnchanged(t *testing.T, dir string, before map[string]string) {
	t.Helper()
	for name, want := range before {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := digestOf(body); got != want {
			t.Errorf("%s changed after a refused update: %s, was %s", name, got, want)
		}
	}
	if stale, err := filepath.Glob(filepath.Join(dir, "*"+previousBundleSuffix)); err == nil && len(stale) > 0 {
		t.Errorf("a refused update left %v behind", stale)
	}
}

func digestOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// bundleZip builds the archive a release publishes.
func bundleZip(t *testing.T, marker string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range bundleFiles(runtime.GOOS) {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(marker + " " + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// releaseServer answers what GitHub answers: one release, the assets it
// publishes, and the files themselves. The whole download path therefore runs
// with no network and no release.
func releaseServer(t *testing.T, tag string, assets map[string][]byte, digests map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/repos/"+release.DefaultRepo+"/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		listed := make([]map[string]any, 0, len(assets))
		for name, body := range assets {
			listed = append(listed, map[string]any{
				"name":                 name,
				"size":                 len(body),
				"browser_download_url": server.URL + "/files/" + name,
				"digest":               digests[name],
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "assets": listed}); err != nil {
			t.Errorf("writing the stub release: %v", err)
		}
	})
	mux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := assets[filepath.Base(r.URL.Path)]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// The first of three refusals: a bundle whose bytes are not the ones the release
// advertises never reaches the installation.
func TestPlayerUpdateRefusesABundleThatFailsItsDigest(t *testing.T) {
	dir := t.TempDir()
	before := writeBundle(t, dir, "installed")
	bundle := bundleZip(t, "new")
	name := release.PlayerName(runtime.GOOS, runtime.GOARCH)
	server := releaseServer(t, "v9.9.9",
		map[string][]byte{name: bundle},
		map[string]string{name: "sha256:" + digestOf([]byte("something else"))})

	target := PlayerTarget{Dir: dir, ExecPath: filepath.Join(dir, playerExecutablePath(runtime.GOOS)), Version: "1.0.0"}
	view, err := ApplyPlayerUpdate(context.Background(), server.Client(), server.URL, target, nil)
	if err == nil {
		t.Fatal("a bundle that failed its digest was accepted")
	}
	if view.State != "failed" || view.Reason != "download_not_verified" {
		t.Errorf("state/reason = %s/%s, want failed/download_not_verified", view.State, view.Reason)
	}
	if view.Current != "1.0.0" {
		t.Errorf("current version = %q, want what is installed", view.Current)
	}
	assertUnchanged(t, dir, before)
}

// The second: a bundle that arrives intact but whose player does not run. The
// digest is right here - the archive is the one this test built - so this is the
// smoke test refusing, which is the step that makes the update safe.
func TestPlayerUpdateRefusesAPlayerThatDoesNotRun(t *testing.T) {
	dir := t.TempDir()
	before := writeBundle(t, dir, "installed")
	bundle := bundleZip(t, "new")
	name := release.PlayerName(runtime.GOOS, runtime.GOARCH)
	server := releaseServer(t, "v9.9.9",
		map[string][]byte{name: bundle},
		map[string]string{name: "sha256:" + digestOf(bundle)})

	target := PlayerTarget{Dir: dir, ExecPath: filepath.Join(dir, playerExecutablePath(runtime.GOOS)), Version: "1.0.0"}
	view, err := ApplyPlayerUpdate(context.Background(), server.Client(), server.URL, target, nil)
	if err == nil {
		t.Fatal("a player that does not run was accepted")
	}
	if view.Reason != "binary_did_not_run" {
		t.Errorf("reason = %q, want binary_did_not_run", view.Reason)
	}
	assertUnchanged(t, dir, before)
}

// A release that publishes no bundle for this platform is a fact about the
// release, not a failure of the tool: it is reported, and nothing is downloaded.
func TestPlayerUpdateReportsAReleaseWithoutABundle(t *testing.T) {
	dir := t.TempDir()
	writeBundle(t, dir, "installed")
	server := releaseServer(t, "v9.9.9",
		map[string][]byte{release.ServerName(runtime.GOOS, runtime.GOARCH): []byte("server")},
		map[string]string{release.ServerName(runtime.GOOS, runtime.GOARCH): "sha256:" + digestOf([]byte("server"))})

	target := PlayerTarget{Dir: dir, ExecPath: filepath.Join(dir, playerExecutablePath(runtime.GOOS)), Version: "1.0.0"}
	view, err := CheckForPlayerUpdate(context.Background(), server.Client(), server.URL, target)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if view.Reason != "no_binary_for_platform" || view.Available {
		t.Errorf("reason/available = %q/%v, want no_binary_for_platform/false", view.Reason, view.Available)
	}
}

// Decision 24, applied to the second program: a build that cannot say what it is
// is refused before anything is asked of the network.
func TestPlayerUpdateRefusesABuildThatCannotNameItself(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer server.Close()

	target := PlayerTarget{Dir: t.TempDir(), ExecPath: "nowhere/theia-player", Version: "dev"}
	view, err := CheckForPlayerUpdate(context.Background(), server.Client(), server.URL, target)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if view.Reason != "development_build" || view.Available {
		t.Errorf("reason/available = %q/%v, want development_build/false", view.Reason, view.Available)
	}
	if reached {
		t.Error("a development build asked the release page anyway")
	}
}

// The swap is only useful if it is all-or-nothing: a bundle that cannot be put
// in place completely must leave every file that was already there exactly as it
// was, including the ones this call had already moved.
func TestSwapBundlePutsEveryFileBackWhenOneCannotBeReplaced(t *testing.T) {
	install := t.TempDir()
	before := writeBundle(t, install, "installed")
	staged := t.TempDir()
	names := bundleFiles(runtime.GOOS)
	for _, name := range names[:len(names)-1] {
		if err := os.WriteFile(filepath.Join(staged, name), []byte("new "+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := swapBundle(staged, install, names); err == nil {
		t.Fatalf("the swap accepted a bundle missing %s", names[len(names)-1])
	}
	assertUnchanged(t, install, before)
}

// And the other half: when every member is there, every one of them is replaced.
// A swap that replaced the executable and left the licence behind would ship a
// player whose notice names the wrong engine.
func TestSwapBundleReplacesEveryFileOfTheBundle(t *testing.T) {
	install := t.TempDir()
	writeBundle(t, install, "installed")
	staged := t.TempDir()
	names := bundleFiles(runtime.GOOS)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(staged, name), []byte("new "+name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := swapBundle(staged, install, names); err != nil {
		t.Fatalf("swapBundle: %v", err)
	}
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(install, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "new "+name {
			t.Errorf("%s = %q, want the staged file", name, body)
		}
	}
	// And nothing moved aside survives a successful update: one of those files is
	// the engine, and leaving a hundred megabytes behind per update is not a
	// design, it is a leak.
	if stale, err := filepath.Glob(filepath.Join(install, "*"+previousBundleSuffix)); err == nil && len(stale) > 0 {
		t.Errorf("a successful update left %v behind", stale)
	}
}
