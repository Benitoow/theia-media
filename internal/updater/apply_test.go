package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Benitoow/theia-media/internal/activity"
)

// helperSource is a stand-in for Theia itself: something that really is an
// executable and really answers -version. Fixtures made of random bytes would
// let the smoke test pass vacuously, which is the one step worth proving.
const helperSource = `package main

import (
	"flag"
	"fmt"
)

var version = "0.0.0"

func main() {
	v := flag.Bool("version", false, "")
	flag.Parse()
	if *v {
		fmt.Println("theia", version)
	}
}
`

// buildHelper compiles the stand-in at a given version and returns its path.
func buildHelper(t *testing.T, version string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(helperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module helper\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "helper")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+version, "-o", out, ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build the test helper (no Go toolchain available?): %v\n%s", err, output)
	}
	return out
}

func digestOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// stubGitHub serves a release whose only asset is the file at binaryPath,
// advertised with the digest given -- which the tests deliberately get wrong.
func stubGitHub(t *testing.T, tag, binaryPath, advertisedDigest string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var server *httptest.Server

	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		rel := release{
			TagName: tag,
			HTMLURL: "https://example.invalid/releases/" + tag,
			Assets: []asset{{
				Name:               assetName(runtime.GOOS, runtime.GOARCH),
				BrowserDownloadURL: server.URL + "/download",
				Digest:             "sha256:" + advertisedDigest,
			}},
		}
		json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, binaryPath)
	})

	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// installation lays out a temporary "installed" Theia at version 1.0.0.
type installation struct {
	execPath string
	original []byte
}

func newInstallation(t *testing.T) installation {
	t.Helper()

	current := buildHelper(t, "1.0.0")
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	execPath := filepath.Join(dir, "theia")
	if runtime.GOOS == "windows" {
		execPath += ".exe"
	}
	if err := os.WriteFile(execPath, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return installation{execPath: execPath, original: data}
}

// assertUntouched is the assertion the whole milestone exists for.
func (i installation) assertUntouched(t *testing.T) {
	t.Helper()

	got, err := os.ReadFile(i.execPath)
	if err != nil {
		t.Fatalf("the installed binary is gone after a failed update: %v", err)
	}
	if string(got) != string(i.original) {
		t.Error("the installed binary was modified by a failed update")
	}
	if _, err := os.Stat(i.execPath + oldSuffix); err == nil {
		t.Error("a failed update left a .old file behind")
	}

	// Nothing half-written may survive either.
	entries, err := os.ReadDir(filepath.Dir(i.execPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) > 6 && e.Name()[:6] == ".theia" {
			t.Errorf("a failed update left the staging file %s behind", e.Name())
		}
	}
}

func newUpdater(t *testing.T, inst installation, apiBase string, tracker *activity.Tracker, restart func()) *Updater {
	t.Helper()
	return New(Options{
		Repo:     "Benitoow/theia-media",
		Version:  "1.0.0",
		ExecPath: inst.execPath,
		Activity: tracker,
		Logger:   slog.New(slog.DiscardHandler),
		Restart:  restart,
		APIBase:  apiBase,
	})
}

func TestApplyInstallsAVerifiedRelease(t *testing.T) {
	inst := newInstallation(t)
	newBinary := buildHelper(t, "1.1.0")
	server := stubGitHub(t, "v1.1.0", newBinary, digestOf(t, newBinary))

	restarted := make(chan struct{}, 1)
	u := newUpdater(t, inst, server.URL, activity.New(), func() { restarted <- struct{}{} })

	if err := u.Apply(t.Context()); err != nil {
		t.Fatalf("Apply returned an unexpected error: %v", err)
	}

	installed, err := os.ReadFile(inst.execPath)
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := os.ReadFile(newBinary)
	if string(installed) != string(expected) {
		t.Error("the installed binary is not the one that was downloaded")
	}

	// The outgoing binary is kept: Windows cannot delete a running executable,
	// and it is the way back if the new version misbehaves.
	if _, err := os.Stat(inst.execPath + oldSuffix); err != nil {
		t.Errorf("the previous binary was not kept: %v", err)
	}

	if status := u.Status(); status.State != StateReady {
		t.Errorf("state = %q, want %q", status.State, StateReady)
	}
	select {
	case <-restarted:
	default:
		t.Error("no restart was requested after a successful update")
	}
}

func TestApplyLeavesTheBinaryIntactOnChecksumMismatch(t *testing.T) {
	// The failure that matters most. A corrupted or tampered download must not
	// reach the disk, and what is installed must be exactly what was there.
	inst := newInstallation(t)
	newBinary := buildHelper(t, "1.1.0")

	wrong := "0000000000000000000000000000000000000000000000000000000000000000"
	server := stubGitHub(t, "v1.1.0", newBinary, wrong)

	u := newUpdater(t, inst, server.URL, activity.New(), func() {
		t.Error("a restart was requested after a failed verification")
	})

	err := u.Apply(t.Context())
	if err == nil {
		t.Fatal("Apply succeeded despite a checksum mismatch")
	}
	inst.assertUntouched(t)

	if status := u.Status(); status.State != StateFailed {
		t.Errorf("state = %q, want %q", status.State, StateFailed)
	}
}

func TestApplyLeavesTheBinaryIntactWhenTheDownloadDoesNotRun(t *testing.T) {
	// A correct checksum only proves the bytes match what was published. This
	// covers the rest: a release built for the wrong architecture, or from a
	// broken commit. It has to be caught before the swap, not after.
	inst := newInstallation(t)

	// Advertised as 1.1.0, but built reporting something else -- the smoke test
	// is what notices.
	impostor := buildHelper(t, "9.9.9")
	server := stubGitHub(t, "v1.1.0", impostor, digestOf(t, impostor))

	u := newUpdater(t, inst, server.URL, activity.New(), func() {
		t.Error("a restart was requested after a failed smoke test")
	})

	if err := u.Apply(t.Context()); err == nil {
		t.Fatal("Apply succeeded despite the binary reporting the wrong version")
	}
	inst.assertUntouched(t)
}

func TestApplyLeavesTheBinaryIntactWhenTheDownloadFails(t *testing.T) {
	inst := newInstallation(t)
	newBinary := buildHelper(t, "1.1.0")

	// A release that advertises an asset the server will not serve.
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release{
			TagName: "v1.1.0",
			Assets: []asset{{
				Name:               assetName(runtime.GOOS, runtime.GOARCH),
				BrowserDownloadURL: server.URL + "/gone",
				Digest:             "sha256:" + digestOf(t, newBinary),
			}},
		})
	})
	mux.HandleFunc("/gone", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)

	u := newUpdater(t, inst, server.URL, activity.New(), nil)
	if err := u.Apply(t.Context()); err == nil {
		t.Fatal("Apply succeeded despite the download failing")
	}
	inst.assertUntouched(t)
}

func TestApplyRefusesWhileSomethingIsPlaying(t *testing.T) {
	// Rule two. A film cut short by an improvement is worse than an improvement
	// that waits.
	inst := newInstallation(t)
	newBinary := buildHelper(t, "1.1.0")
	server := stubGitHub(t, "v1.1.0", newBinary, digestOf(t, newBinary))

	tracker := activity.New()
	done := tracker.Begin() // a stream is open
	defer done()

	u := newUpdater(t, inst, server.URL, tracker, func() {
		t.Error("a restart was requested while something was playing")
	})

	err := u.Apply(t.Context())
	if !errors.Is(err, ErrPlaybackInProgress) {
		t.Fatalf("err = %v, want ErrPlaybackInProgress", err)
	}
	inst.assertUntouched(t)

	if status := u.Status(); status.State != StateDeferred {
		t.Errorf("state = %q, want %q", status.State, StateDeferred)
	}
}

func TestApplyRefusesAReleaseWithoutADigest(t *testing.T) {
	// "We could not check it" is not a reason to install it.
	inst := newInstallation(t)
	newBinary := buildHelper(t, "1.1.0")
	server := stubGitHub(t, "v1.1.0", newBinary, "")

	u := newUpdater(t, inst, server.URL, activity.New(), nil)
	if err := u.Apply(t.Context()); err == nil {
		t.Fatal("Apply installed a binary with no published digest")
	}
	inst.assertUntouched(t)
}

func TestApplyDoesNothingWhenAlreadyCurrent(t *testing.T) {
	inst := newInstallation(t)
	newBinary := buildHelper(t, "1.0.0")
	server := stubGitHub(t, "v1.0.0", newBinary, digestOf(t, newBinary))

	u := newUpdater(t, inst, server.URL, activity.New(), func() {
		t.Error("a restart was requested with no update to install")
	})

	if err := u.Apply(t.Context()); err != nil {
		t.Fatalf("Apply returned an unexpected error: %v", err)
	}
	inst.assertUntouched(t)
}

func TestDevelopmentBuildsNeverUpdateThemselves(t *testing.T) {
	// A build that cannot say what version it is has nothing to compare
	// against, and would otherwise overwrite a developer's working binary.
	inst := newInstallation(t)
	u := New(Options{
		Repo:     "Benitoow/theia-media",
		Version:  "dev",
		ExecPath: inst.execPath,
		Logger:   slog.New(slog.DiscardHandler),
	})

	if u.Supported() {
		t.Error("a dev build reports itself as updatable")
	}
	if err := u.Apply(t.Context()); err == nil {
		t.Fatal("Apply ran on a development build")
	}
	inst.assertUntouched(t)
}

func TestCleanPreviousRemovesTheLeftoverBinary(t *testing.T) {
	inst := newInstallation(t)
	previous := inst.execPath + oldSuffix
	if err := os.WriteFile(previous, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	CleanPrevious(inst.execPath, slog.New(slog.DiscardHandler))

	if _, err := os.Stat(previous); !os.IsNotExist(err) {
		t.Errorf("the leftover binary is still there: %v", err)
	}
	if _, err := os.Stat(inst.execPath); err != nil {
		t.Errorf("CleanPrevious removed the live binary: %v", err)
	}
}

func TestSwapRestoresThePreviousBinaryIfInstallingFails(t *testing.T) {
	// The rollback. If the second rename fails, the installation must not be
	// left without an executable at all.
	inst := newInstallation(t)
	u := newUpdater(t, inst, "http://unused.invalid", activity.New(), nil)

	// A staged file that is not there forces the second rename to fail, which
	// is the branch under test.
	staged := filepath.Join(filepath.Dir(inst.execPath), "never-written")

	if err := u.swap(staged); err == nil {
		t.Fatal("swap succeeded with a staged file that does not exist")
	}

	got, err := os.ReadFile(inst.execPath)
	if err != nil {
		t.Fatalf("the executable was not restored: %v", err)
	}
	if string(got) != string(inst.original) {
		t.Error("the restored executable is not the original")
	}
}
