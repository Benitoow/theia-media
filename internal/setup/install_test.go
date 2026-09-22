package setup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/release"
)

// The installation path is where the "one download" promise is kept or broken:
// a person takes one file from the release page, and everything else has to
// arrive, verified, or not at all. These tests drive it against a local folder,
// against an archive, and against a stub release page serving real bytes.

// fakeRelease writes a folder holding what an unpacked archive holds.
func fakeRelease(t *testing.T, dir string, withPlayer bool) {
	t.Helper()
	write(t, filepath.Join(dir, installed("theia-server")), "MZ the server")
	if runtime.GOOS == "windows" {
		// The launcher is published for Windows only, under its own name inside
		// the installation: an archive of this platform holds one, and an
		// archive of any other platform holds none.
		write(t, filepath.Join(dir, installed(launcherBase)), "MZ the launcher")
	}
	if !withPlayer {
		return
	}
	write(t, filepath.Join(dir, installed("theia-player")), "MZ the player")
	for _, member := range bundleExtras() {
		write(t, filepath.Join(dir, member), bundleBody(member))
	}
}

// installedNames is what a role leaves in the installation directory, read from
// the product's own list so that a program added to it cannot be forgotten here.
func installedNames(role Role) []string {
	names := make([]string, 0, 4)
	for _, want := range programsFor(role, runtime.GOOS, runtime.GOARCH) {
		names = append(names, executableName(want))
	}
	return names
}

// installed is the name a program carries once it is in place. It is the
// product's own rule rather than a second copy of it: a server is theia-server
// on every platform and theia-server.exe on Windows, and the rule itself is
// pinned by TestTheInstallerAcceptsTheNamesTheReleasePublishes. Writing .exe
// into a fixture on Linux is how nine of these tests came to fail there while
// passing on the machine they were written on.
func installed(base string) string { return executablePath(base, runtime.GOOS) }

// bundleExtras is what the player's published bundle carries on this platform
// besides the executable itself, read from the product's own list: Windows
// ships the engine and its licences as three flat files, macOS ships the
// application tree the engine and the licences live inside, and the tests that
// need a second member say so instead of pretending one list is universal.
func bundleExtras() []string { return bundleFiles(runtime.GOOS)[1:] }

// bundleBody is what a fixture writes into a bundle member, so an assertion can
// prove the member came out of the bundle rather than from somewhere else.
func bundleBody(member string) string {
	switch member {
	case "libmpv-2.dll", darwinPlayerEngine:
		return "engine"
	case "LICENSE-libmpv.txt", darwinPlayerLicence:
		return "LGPL"
	case "NOTICE.md", darwinPlayerNotice:
		return "notice"
	}
	return "bundle member"
}

// needsBundleExtras skips a test whose whole point is an incomplete bundle.
// There is nothing to leave out of a bundle that is one file.
func needsBundleExtras(t *testing.T) {
	t.Helper()
	if len(bundleExtras()) == 0 {
		t.Skipf("the %s player bundle is the executable alone; no member can be missing from it", runtime.GOOS)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(body)
}

// recording is a Reporter that keeps what it was told.
type recording struct {
	phases   []Phase
	details  []string
	progress int
	last     int64
	total    int64
}

func (r *recording) Phase(phase Phase, detail string) {
	r.phases = append(r.phases, phase)
	r.details = append(r.details, detail)
}

func (r *recording) Progress(done, total int64) {
	r.progress++
	r.last, r.total = done, total
}

func TestAReleaseFolderBesideTheInstallerIsInstalledWhole(t *testing.T) {
	// What a person has after extracting the archive: every file in one folder.
	// The installer copies them into the installation and keeps the licences
	// with the engine, because a player without its notice is a licence breach.
	source := t.TempDir()
	fakeRelease(t, source, true)
	install := t.TempDir()

	plan := Plan{Role: RoleAllInOne, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}
	report := &recording{}
	actions, err := InstallPrograms(context.Background(), plan, Source{From: source}, report)
	if err != nil {
		t.Fatalf("InstallPrograms: %v", err)
	}
	for _, name := range append(installedNames(RoleAllInOne), bundleExtras()...) {
		if _, err := os.Stat(filepath.Join(install, name)); err != nil {
			t.Errorf("%s was not installed: %v", name, err)
		}
	}
	if read(t, filepath.Join(install, installed("theia-server"))) != "MZ the server" {
		t.Error("the installed server is not the file that was copied")
	}
	if want := len(installedNames(RoleAllInOne)); len(actions) != want {
		t.Errorf("the installation reported %d programs, want %d: %+v", len(actions), want, actions)
	}
	if report.phases[0] != PhaseChecking || report.phases[len(report.phases)-1] != PhaseDone {
		t.Errorf("the phases do not begin and end where they should: %v", report.phases)
	}
	// Nothing was downloaded, so nothing needed to say how many bytes.
	if report.progress != 0 {
		t.Errorf("a local installation reported %d progress updates", report.progress)
	}
}

func TestAReleaseArchiveIsInstalledWithoutUnpackingItFirst(t *testing.T) {
	// The other thing a person has: the zip itself, untouched, because Windows
	// opens archives rather than extracting them half the time.
	archive := filepath.Join(t.TempDir(), "theia-3.3.0-"+runtime.GOOS+"-"+runtime.GOARCH+".zip")
	members := map[string]string{"START-HERE.txt": "read me first"}
	for _, name := range installedNames(RoleAllInOne) {
		members[name] = "MZ " + name
	}
	for _, member := range bundleExtras() {
		members[member] = bundleBody(member)
	}
	makeZip(t, archive, members)
	install := t.TempDir()

	plan := Plan{Role: RoleAllInOne, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}
	if _, err := InstallPrograms(context.Background(), plan, Source{From: archive}, nil); err != nil {
		t.Fatalf("InstallPrograms from an archive: %v", err)
	}
	for _, name := range append(installedNames(RoleAllInOne), bundleExtras()...) {
		if _, err := os.Stat(filepath.Join(install, name)); err != nil {
			t.Errorf("%s was not installed from the archive: %v", name, err)
		}
	}
	// Only what the programs need: the note to the reader is not part of an
	// installation.
	if _, err := os.Stat(filepath.Join(install, "START-HERE.txt")); err == nil {
		t.Error("the archive's reading note was installed as if it were a program")
	}
}

func TestAnIncompletePlayerBundleStopsTheInstallation(t *testing.T) {
	// The player looks installed without its engine and does not start. Half a
	// bundle is a failure, and the failure names what was missing.
	needsBundleExtras(t)
	source := t.TempDir()
	write(t, filepath.Join(source, installed("theia-player")), "MZ the player")
	install := t.TempDir()

	plan := Plan{Role: RolePlayer, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}
	_, err := InstallPrograms(context.Background(), plan, Source{From: source}, nil)
	if err == nil {
		t.Fatal("an installation with no engine was accepted")
	}
	var failure *InstallError
	if !errors.As(err, &failure) {
		t.Fatalf("the failure carries no reason: %v", err)
	}
	if failure.Reason != ReasonIncompleteBundle {
		t.Errorf("reason = %q, want %q", failure.Reason, ReasonIncompleteBundle)
	}
	if !strings.Contains(failure.Detail, "libmpv") {
		t.Errorf("the failure does not name the missing file: %q", failure.Detail)
	}
}

func TestAnInstalledProgramIsNotDownloadedAgain(t *testing.T) {
	// Running the installer twice is normal - that is how somebody adds a
	// folder - and it must not fetch a hundred megabytes a second time. The
	// release page is not even asked: the stub below fails the test if it is.
	asked := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = true
		http.Error(w, "the release page should not have been asked", http.StatusInternalServerError)
	}))
	defer server.Close()

	install := t.TempDir()
	for _, name := range installedNames(RoleServer) {
		write(t, filepath.Join(install, name), "MZ already here")
	}
	plan := Plan{Role: RoleServer, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}

	actions, err := InstallPrograms(context.Background(), plan, Source{APIBase: server.URL}, nil)
	if err != nil {
		t.Fatalf("InstallPrograms: %v", err)
	}
	if asked {
		t.Error("the installer asked the release page for a program it already had")
	}
	if want := len(installedNames(RoleServer)); len(actions) != want {
		t.Errorf("the installation reported %d programs, want %d: %+v", len(actions), want, actions)
	}
	for _, action := range actions {
		if action.Detail != "already-installed" {
			t.Errorf("the action does not say the program was already there: %+v", action)
		}
	}
}

func TestTheInstallerFetchesAndVerifiesWhatIsMissing(t *testing.T) {
	// The single-download promise: the release page publishes the installer, and
	// the installer fetches the rest itself. Everything here is real bytes with
	// real digests, served by a stub standing in for GitHub.
	bundle := filepath.Join(t.TempDir(), "player.zip")
	player := map[string]string{installed("theia-player"): "MZ the player"}
	for _, member := range bundleExtras() {
		player[member] = bundleBody(member)
	}
	makeZip(t, bundle, player)
	payloads := map[string][]byte{}
	for _, want := range programsFor(RoleAllInOne, runtime.GOOS, runtime.GOARCH) {
		if want.bundle {
			payloads[want.asset] = mustRead(t, bundle)
			continue
		}
		payloads[want.asset] = []byte("MZ " + want.base + ", downloaded")
	}
	server := stubReleasePage(t, "v3.3.0", payloads)

	install := t.TempDir()
	plan := Plan{Role: RoleAllInOne, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}
	report := &recording{}
	actions, err := InstallPrograms(context.Background(), plan, Source{APIBase: server.URL, Client: server.Client()}, report)
	if err != nil {
		t.Fatalf("InstallPrograms: %v", err)
	}
	if read(t, filepath.Join(install, installed("theia-server"))) != "MZ theia-server, downloaded" {
		t.Error("the server was not installed from the release")
	}
	for _, member := range bundleExtras() {
		if read(t, filepath.Join(install, member)) != bundleBody(member) {
			t.Errorf("%s did not come out of the published bundle", member)
		}
	}
	for _, action := range actions {
		if action.Kind != "downloaded-program" || action.Detail != "v3.3.0" {
			t.Errorf("the action does not name the release it came from: %+v", action)
		}
	}
	// The bar is only useful if it is fed, and only during a download.
	if report.progress == 0 {
		t.Error("the download reported no progress at all")
	}
	// The number on the bar is one of the payloads it fetched, and a download
	// with no size at all (-1) would leave the bar empty.
	sizes := map[int64]bool{}
	for _, payload := range payloads {
		sizes[int64(len(payload))] = true
	}
	if !sizes[report.total] {
		t.Errorf("progress total = %d, which is none of the payloads", report.total)
	}
	var sawDownload, sawExtract bool
	for _, phase := range report.phases {
		sawDownload = sawDownload || phase == PhaseDownloading
		sawExtract = sawExtract || phase == PhaseExtracting
	}
	if !sawDownload || !sawExtract {
		t.Errorf("the phases say nothing about downloading or extracting: %v", report.phases)
	}
	// And nothing of the staging survives in the installation.
	if leftovers, _ := filepath.Glob(filepath.Join(install, "*.part")); len(leftovers) != 0 {
		t.Errorf("a partial file was left in the installation: %v", leftovers)
	}
}

func TestAFailedDownloadLeavesNothingInstalled(t *testing.T) {
	// The rule the founding spec states outright: no unverified binary reaches a
	// disk. A payload that does not match its digest leaves no program behind.
	server := stubReleasePage(t, "v3.3.0", map[string][]byte{
		release.ServerName(runtime.GOOS, runtime.GOARCH): []byte("MZ the honest server"),
	})
	// The stub advertises the digest of other bytes.
	corrupt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			body := map[string]any{
				"tag_name": "v3.3.0",
				"assets": []map[string]any{{
					"name":                 release.ServerName(runtime.GOOS, runtime.GOARCH),
					"size":                 len("MZ something else entirely"),
					"browser_download_url": server.URL + "/asset",
					"digest":               "sha256:" + strings.Repeat("0", 64),
				}},
			}
			json.NewEncoder(w).Encode(body)
			return
		}
		w.Write([]byte("MZ something else entirely"))
	}))
	defer corrupt.Close()

	install := t.TempDir()
	plan := Plan{Role: RoleServer, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}
	_, err := InstallPrograms(context.Background(), plan, Source{APIBase: corrupt.URL, Client: corrupt.Client()}, nil)
	if err == nil {
		t.Fatal("a download that failed its digest was installed")
	}
	entries, _ := os.ReadDir(install)
	for _, entry := range entries {
		t.Errorf("a refused installation left %s behind", entry.Name())
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
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
}

// stubReleasePage stands in for GitHub: the JSON document naming the assets and
// their digests, and the bytes at the addresses it gives them.
func stubReleasePage(t *testing.T, tag string, payloads map[string][]byte) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			rel := map[string]any{"tag_name": tag, "html_url": server.URL + "/tag"}
			assets := make([]map[string]any, 0, len(payloads))
			for name, body := range payloads {
				sum := sha256.Sum256(body)
				assets = append(assets, map[string]any{
					"name":                 name,
					"size":                 len(body),
					"browser_download_url": server.URL + "/assets/" + name,
					"digest":               "sha256:" + hex.EncodeToString(sum[:]),
				})
			}
			rel["assets"] = assets
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
	return server
}

// TestTheLauncherIsInstalledWhereItIsPublished pins the two names apart. The
// `theia` command is installed under its own name and published under one that
// says what it is - never `theia-windows-amd64.exe`, which was the server's
// retired alias (decision 119) and is still sitting in folders people
// downloaded a v3.2 release into.
func TestTheLauncherIsInstalledWhereItIsPublished(t *testing.T) {
	wanted := programsFor(RoleAllInOne, "windows", "amd64")
	last := wanted[len(wanted)-1]
	if last.base != "theia" || last.asset != release.LauncherName("windows", "amd64") {
		t.Fatalf("the Windows installation ends with %+v", last)
	}
	if names := acceptedNames(last); !slices.Contains(names, "theia.exe") ||
		!slices.Contains(names, release.LauncherName("windows", "amd64")) {
		t.Errorf("the launcher accepts %v", names)
	} else if slices.Contains(names, "theia-windows-amd64.exe") {
		t.Error("the launcher accepts the retired server alias name")
	}

	// Only where it is published. The launcher is a Windows and macOS program,
	// like the player: a platform the release publishes neither for gets neither,
	// and asking for an asset that is not there is a failed installation rather
	// than a missing feature.
	for _, platform := range []string{"linux"} {
		for _, entry := range programsFor(RoleAllInOne, platform, "amd64") {
			if entry.base == launcherBase {
				t.Errorf("the %s installation asks for a launcher", platform)
			}
		}
	}
	for _, platform := range []string{"windows", "darwin"} {
		found := false
		for _, entry := range programsFor(RoleAllInOne, platform, "amd64") {
			if entry.base == launcherBase {
				found = true
			}
		}
		if !found {
			t.Errorf("the %s installation does not ask for a launcher", platform)
		}
	}
}

// TestTheInstallationDirectoryIsPerUser guards the promise that this installer
// never asks for administrator rights: a machine-wide directory would need them.
func TestTheInstallationDirectoryIsPerUser(t *testing.T) {
	dir, err := DefaultInstallDir()
	if err != nil {
		t.Skipf("this machine has no per-user directory to speak of: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("the installation directory is not absolute: %q", dir)
	}
	if strings.Contains(strings.ToLower(dir), "program files") {
		t.Errorf("the installation directory needs administrator rights: %q", dir)
	}
	if !bytes.Contains([]byte(strings.ToLower(dir)), []byte("theia")) {
		t.Errorf("the installation directory does not name the product: %q", dir)
	}
}
