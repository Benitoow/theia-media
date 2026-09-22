package setup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/release"
)

// The macOS shape of an installation, as far as it can be driven on a Windows
// machine.
//
// Everything platform-specific here is a function of a `goos` argument or of the
// home directory, so the answers macOS gets are pinned by the suite that runs
// here. What is *not* covered is the half only a Mac can answer: that a symlink in
// ~/Applications opens the player from Finder or Spotlight, and that launchd
// accepts the agent. Those are reported as unverified rather than claimed.

// macPlayer is the darwin player as the installer describes it.
func macPlayer() program { return playerProgram("darwin", "arm64") }

// macPlayerBundle is the asset the release publishes for macOS: a zip whose
// members are the application tree, including the directory entry an archiver
// writes for the bundle itself. Members named in omit are left out, which is how
// an incomplete bundle is built.
func macPlayerBundle(t *testing.T, omit ...string) string {
	t.Helper()
	members := map[string]string{
		darwinPlayerTree + "/": "",
		darwinPlayerExecutable: "MZ the player",
		darwinPlayerEngine:     bundleBody(darwinPlayerEngine),
		darwinPlayerLicence:    bundleBody(darwinPlayerLicence),
		darwinPlayerNotice:     bundleBody(darwinPlayerNotice),
	}
	for _, member := range omit {
		delete(members, member)
	}
	path := filepath.Join(t.TempDir(), release.PlayerName("darwin", "arm64"))
	makeZip(t, path, members)
	return path
}

// macPlayerFolder is the same bundle as a person has after extracting the zip
// themselves, which is what `--from <folder>` is handed. Members named in omit
// are left out, exactly as in macPlayerBundle.
func macPlayerFolder(t *testing.T, omit ...string) string {
	t.Helper()
	dir := t.TempDir()
	if slices.Contains(omit, darwinPlayerTree) {
		return dir
	}
	write(t, filepath.Join(dir, filepath.FromSlash(darwinPlayerExecutable)), "MZ the player")
	for _, member := range []string{darwinPlayerEngine, darwinPlayerLicence, darwinPlayerNotice} {
		if slices.Contains(omit, member) {
			continue
		}
		write(t, filepath.Join(dir, filepath.FromSlash(member)), bundleBody(member))
	}
	return dir
}

// TestTheMacintoshProgramsAreWhatTheReleasePublishes pins the whole set at once:
// the three programs a macOS installation wants, the names a sentence calls them
// by, the paths they have on disk, and the assets they come from. A change to any
// of them is a change to what gets installed.
func TestTheMacintoshProgramsAreWhatTheReleasePublishes(t *testing.T) {
	wanted := programsFor(RoleAllInOne, "darwin", "arm64")
	if len(wanted) != 3 {
		t.Fatalf("a macOS installation wants %d programs, want 3: %+v", len(wanted), wanted)
	}

	server, player, launcher := wanted[0], wanted[1], wanted[2]
	if server.label != "server" || server.installed != "theia-server" ||
		server.asset != release.ServerName("darwin", "arm64") {
		t.Errorf("the server is %+v", server)
	}
	if launcher.label != "theia" || launcher.installed != "theia" ||
		launcher.asset != release.LauncherName("darwin", "arm64") {
		t.Errorf("the launcher is %+v", launcher)
	}
	if player.label != "player" || player.asset != release.PlayerName("darwin", "arm64") {
		t.Errorf("the player is %+v", player)
	}
	// The player is the one program whose disk path is not its name, which is the
	// whole reason the two are separate fields.
	if player.installed != darwinPlayerExecutable || player.tree != darwinPlayerTree {
		t.Errorf("the player installs as %q in %q", player.installed, player.tree)
	}
	if !player.bundle {
		t.Error("the macOS player is not described as a bundle")
	}
	// Both names are real on disk: the bundle, and the executable inside it.
	if names := acceptedNames(player); len(names) != 2 || names[0] != darwinPlayerTree {
		t.Errorf("the player is found as %v", names)
	}

	// A label is read by the interface, so every one of them has to exist as a
	// sentence in both languages - the failure would otherwise be an empty space
	// in the progress bar, in whichever language is missing it.
	for _, platform := range []string{"darwin", "windows", "linux"} {
		for _, want := range programsFor(RoleAllInOne, platform, "amd64") {
			key := "program." + want.Label()
			if strings.TrimSpace(english[key]) == "" || strings.TrimSpace(french[key]) == "" {
				t.Errorf("the label %q is not in both languages", want.Label())
			}
		}
	}
}

// TestTheMacintoshPlayerBundleIsAnApplicationTree pins the members the release
// has to publish and the installer has to find. The tree comes first because it
// is what an extraction is asked for by name, and every member after it is what
// makes "the engine is missing" a refusal rather than an installation.
func TestTheMacintoshPlayerBundleIsAnApplicationTree(t *testing.T) {
	want := []string{
		"Theia.app",
		"Theia.app/Contents/MacOS/theia-player",
		"Theia.app/Contents/Frameworks/libmpv.dylib",
		"Theia.app/Contents/Resources/LICENSE-libmpv.txt",
		"Theia.app/Contents/Resources/NOTICE.md",
	}
	got := bundleFiles("darwin")
	if len(got) != len(want) {
		t.Fatalf("the macOS bundle is %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("the macOS bundle is %v, want %v", got, want)
		}
	}
	// The executable the bundle must hold is the path the program installs as,
	// which is what keeps "the player is installed" and "the tree is complete"
	// from being two different answers.
	if name := executableName(macPlayer()); !slices.Contains(want, name) {
		t.Errorf("the player installs as %q, which is not one of %v", name, want)
	}
	// And Windows is untouched by any of this: four flat files beside each other.
	if windows := bundleFiles("windows"); len(windows) != 4 || windows[0] != "theia-player.exe" {
		t.Errorf("the Windows bundle is now %v", windows)
	}
}

// TestTheMacintoshPlayerIsInstalledAsAnApplicationTree drives the two local
// sources a person actually has - the archive and the folder they extracted it
// into - and checks that what lands is a bundle the system would accept.
func TestTheMacintoshPlayerIsInstalledAsAnApplicationTree(t *testing.T) {
	player := macPlayer()
	for _, source := range []struct {
		name string
		path string
	}{
		{"the published archive", macPlayerBundle(t)},
		{"an extracted folder", macPlayerFolder(t)},
	} {
		install := t.TempDir()
		plan := Plan{Role: RolePlayer, DataDir: t.TempDir(), InstallDir: install, Port: 8395, Hostname: "theia"}

		var err error
		if strings.EqualFold(filepath.Ext(source.path), ".zip") {
			err = placeFromArchive(source.path, plan, player)
		} else {
			err = placeFromFolder(source.path, plan, player)
		}
		if err != nil {
			t.Fatalf("installing from %s: %v", source.name, err)
		}

		if !isDirectory(filepath.Join(install, darwinPlayerTree)) {
			t.Errorf("installing from %s did not produce an application", source.name)
		}
		for _, member := range player.files {
			if !pathExists(filepath.Join(install, filepath.FromSlash(member))) {
				t.Errorf("%s is missing after installing from %s", member, source.name)
			}
		}
		if body := read(t, filepath.Join(install, filepath.FromSlash(darwinPlayerExecutable))); body != "MZ the player" {
			t.Errorf("the installed executable is %q", body)
		}
		// The engine came out of the bundle rather than from somewhere else, and
		// the licence followed it: shipping libmpv without its notice is a licence
		// breach, not an incomplete download.
		if body := read(t, filepath.Join(install, filepath.FromSlash(darwinPlayerEngine))); body != "engine" {
			t.Errorf("the installed engine is %q", body)
		}
		if body := read(t, filepath.Join(install, filepath.FromSlash(darwinPlayerLicence))); body != "LGPL" {
			t.Errorf("the installed licence is %q", body)
		}
		// And the installation says it is complete, which is what stops a second
		// run of the installer from downloading it all again.
		if !hasProgram(install, player) {
			t.Errorf("the installation made from %s does not look installed", source.name)
		}
		// `--check` finds an installed program with a file test over the accepted
		// names, so the executable inside the bundle has to be one of them: a
		// directory is not a file, and the tree alone would report every macOS
		// installation as missing its player.
		names := acceptedNames(player)
		if !fileExists(filepath.Join(install, filepath.FromSlash(names[len(names)-1]))) {
			t.Errorf("the last accepted name %q is not a file in the installation", names[len(names)-1])
		}
	}
}

// TestTheMacintoshMaintenanceToolKeepsItsBareName pins the pair a published name
// and an installed name have to keep apart: the release publishes the tool as
// `theia-setup-darwin-arm64` (release.SetupName, pinned in internal/release), and
// the copy that travels with the installation is `theia-setup` - the name
// `--uninstall` is reached by, and the name the macOS command entry links to.
func TestTheMacintoshMaintenanceToolKeepsItsBareName(t *testing.T) {
	if got := installerExecutable("darwin"); got != "theia-setup" {
		t.Errorf("the macOS maintenance tool is installed as %q", got)
	}
	// Windows is the only platform whose extension a bare name carries, and it is
	// unchanged: an installation that renamed its own tool would break the
	// uninstall command the applications list already holds.
	if got := installerExecutable("windows"); got != "theia-setup.exe" {
		t.Errorf("the Windows maintenance tool is installed as %q", got)
	}
}

// TestAnIncompleteMacintoshPlayerStopsTheInstallation is the rule that matters
// most about a bundle: the player looks installed without its engine and does not
// start, and shipping one without its licence is not an installation.
func TestAnIncompleteMacintoshPlayerStopsTheInstallation(t *testing.T) {
	player := macPlayer()
	plan := Plan{Role: RolePlayer, DataDir: t.TempDir(), InstallDir: t.TempDir(), Port: 8395, Hostname: "theia"}

	for _, missing := range []string{darwinPlayerEngine, darwinPlayerLicence} {
		fromArchive := Plan{Role: RolePlayer, DataDir: t.TempDir(), InstallDir: t.TempDir()}
		err := placeFromArchive(macPlayerBundle(t, missing), fromArchive, player)
		assertIncomplete(t, err, missing, "an archive")

		fromFolder := Plan{Role: RolePlayer, DataDir: t.TempDir(), InstallDir: t.TempDir()}
		err = placeFromFolder(macPlayerFolder(t, missing), fromFolder, player)
		assertIncomplete(t, err, missing, "a folder")
	}

	// And an installation that lost one of its members stops looking installed,
	// which is what makes the next run repair it rather than skip it.
	if err := placeFromArchive(macPlayerBundle(t), plan, player); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(plan.InstallDir, filepath.FromSlash(darwinPlayerEngine))); err != nil {
		t.Fatal(err)
	}
	if hasProgram(plan.InstallDir, player) {
		t.Error("a player with no engine is reported as installed")
	}
}

func assertIncomplete(t *testing.T, err error, member, source string) {
	t.Helper()
	if err == nil {
		t.Fatalf("an incomplete player from %s was accepted", source)
	}
	var failure *InstallError
	if !errors.As(err, &failure) {
		t.Fatalf("the failure from %s carries no reason: %v", source, err)
	}
	if failure.Reason != ReasonIncompleteBundle {
		t.Errorf("reason from %s = %q, want %q", source, failure.Reason, ReasonIncompleteBundle)
	}
	if !strings.Contains(err.Error(), member) {
		t.Errorf("the failure from %s does not name the missing member: %v", source, err)
	}
}

// TestTheMacintoshPlayerIsDownloadedAndVerifiedLikeAnyOther drives the real
// download path with a macOS payload: the release page, the digest, the
// extraction, all against a stub serving real bytes.
func TestTheMacintoshPlayerIsDownloadedAndVerifiedLikeAnyOther(t *testing.T) {
	archive := macPlayerBundle(t)
	payload, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	name := release.PlayerName("darwin", "arm64")
	server := stubReleasePage(t, "v3.3.0", map[string][]byte{name: payload})

	sum := sha256.Sum256(payload)
	asset := release.Asset{
		Name:   name,
		Size:   int64(len(payload)),
		URL:    server.URL + "/assets/" + name,
		Digest: "sha256:" + hex.EncodeToString(sum[:]),
	}

	plan := Plan{Role: RolePlayer, DataDir: t.TempDir(), InstallDir: t.TempDir(), Port: 8395, Hostname: "theia"}
	player := macPlayer()
	source := Source{APIBase: server.URL, Client: server.Client()}
	if err := fetchProgram(context.Background(), plan, source, player, asset, "v3.3.0", nil); err != nil {
		t.Fatalf("fetchProgram: %v", err)
	}
	if !hasProgram(plan.InstallDir, player) {
		t.Error("the downloaded macOS player is not a complete installation")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(plan.InstallDir, "*.part")); len(leftovers) != 0 {
		t.Errorf("a partial file was left in the installation: %v", leftovers)
	}
}

// TestTheMacintoshEntriesPointIntoTheInstallation covers the two entries a Mac
// gets instead of a Start Menu: the application in ~/Applications, which is what
// Finder, Spotlight and Launchpad read, and the command in ~/.local/bin.
//
// The symlink itself is passed in, because a Windows machine will not let an
// ordinary process create one without a privilege - and what these entries say is
// the part that can be wrong.
func TestTheMacintoshEntriesPointIntoTheInstallation(t *testing.T) {
	home, install := t.TempDir(), t.TempDir()
	write(t, filepath.Join(install, filepath.FromSlash(darwinPlayerExecutable)), "MZ the player")
	write(t, filepath.Join(install, launcherBase), "MZ the launcher")

	made := map[string]string{}
	makeLink := func(target, name string) error {
		made[name] = target
		return nil
	}

	actions, failure := linkMacEntries(macEntries(RoleAllInOne, home, install), makeLink)
	if failure != "" {
		t.Fatalf("the entries were not written: %s", failure)
	}
	appLink := filepath.Join(home, "Applications", "Theia.app")
	commandLink := filepath.Join(home, ".local", "bin", launcherBase)
	want := []struct{ link, target string }{
		{appLink, filepath.Join(install, "Theia.app")},
		{commandLink, filepath.Join(install, launcherBase)},
	}
	for _, entry := range want {
		if made[entry.link] != entry.target {
			t.Errorf("%s points at %q, want %q", entry.link, made[entry.link], entry.target)
		}
	}
	if len(made) != len(want) {
		t.Errorf("the installation wrote %d entries, want %d: %v", len(made), len(want), made)
	}
	kinds := map[string]int{}
	for _, action := range actions {
		if action.Kind != "created-shortcut" {
			t.Errorf("the action is %q, which the interface cannot render", action.Kind)
		}
		kinds[action.Detail]++
	}
	if kinds["application"] != 1 || kinds["command"] != 1 {
		t.Errorf("the entries were reported as %v", kinds)
	}

	// A machine that never installs the player gets no application entry: an
	// entry pointing at a program that is not there is a broken promise somebody
	// double-clicks. The command entry is written whatever the role is, because
	// the command is installed whatever the role is.
	for _, role := range []Role{RoleServer, RolePlayer, RoleAllInOne} {
		entries := macEntries(role, home, install)
		wantApp := role.WantsPlayer()
		foundApp := false
		for _, entry := range entries {
			if entry.Detail == "application" {
				foundApp = true
			}
		}
		if foundApp != wantApp {
			t.Errorf("the %s installation %s an application entry", role, map[bool]string{true: "wants", false: "does not want"}[wantApp])
		}
		if len(entries) == 0 {
			t.Errorf("the %s installation leaves no entry at all", role)
		}
	}
}

func TestAMacintoshEntryPointingAtNothingIsReported(t *testing.T) {
	// The other half of "no entry without a program": when the role wants an
	// application and the installation has none, the failure says what is
	// missing instead of writing an entry that leads nowhere.
	home, install := t.TempDir(), t.TempDir()
	write(t, filepath.Join(install, launcherBase), "MZ the launcher")

	made := 0
	actions, failure := linkMacEntries(macEntries(RoleAllInOne, home, install), func(target, name string) error {
		made++
		return nil
	})
	if failure == "" {
		t.Fatal("an entry pointing at a missing application was written")
	}
	if !strings.Contains(failure, "Theia.app") {
		t.Errorf("the failure does not say what was missing: %q", failure)
	}
	if made != 1 {
		t.Errorf("%d entries were written, want only the command", made)
	}
	if len(actions) != 1 {
		t.Errorf("the installation reported %+v", actions)
	}
}

func TestSomethingElseAtTheEntryPathIsLeftAlone(t *testing.T) {
	// A re-install replaces the symlink it wrote last time, because a symlink is
	// the installer's own kind of object. A folder somebody put there themselves
	// is not, and deleting it would be removing something this tool did not make.
	home, install := t.TempDir(), t.TempDir()
	write(t, filepath.Join(install, filepath.FromSlash(darwinPlayerExecutable)), "MZ the player")
	write(t, filepath.Join(install, launcherBase), "MZ the launcher")
	if err := os.MkdirAll(macApplicationLink(home), 0o755); err != nil {
		t.Fatal(err)
	}

	made := 0
	_, failure := linkMacEntries(macEntries(RoleAllInOne, home, install), func(target, name string) error {
		made++
		return nil
	})
	if failure == "" {
		t.Error("something that is not a symlink was silently replaced")
	}
	if made != 1 {
		t.Errorf("%d entries were written, want only the command", made)
	}
	if !isDirectory(macApplicationLink(home)) {
		t.Error("a folder this installer did not write was removed")
	}
}

func TestRemovingTheMacintoshEntriesTakesTheLinks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	// Plain files stand in for the symlinks: what the removal acts on is the
	// entry path, and a Windows machine cannot always create a real symlink.
	for _, link := range []string{macApplicationLink(home), macCommandLink(home)} {
		write(t, link, "a link")
	}
	// And one entry path holding something the installer did not write.
	occupied := macCommandLink(home)
	if err := os.Remove(occupied); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(occupied, "somebody-elses"), 0o755); err != nil {
		t.Fatal(err)
	}

	actions := unlinkMacEntries([]string{macApplicationLink(home), macCommandLink(home)})
	if len(actions) != 1 || actions[0].Kind != "removed-shortcut" {
		t.Fatalf("the uninstall reported %+v", actions)
	}
	if actions[0].Path != macApplicationLink(home) {
		t.Errorf("the removed entry is %q", actions[0].Path)
	}
	if _, err := os.Stat(macApplicationLink(home)); !os.IsNotExist(err) {
		t.Errorf("the application entry is still there: %v", err)
	}
	if !isDirectory(occupied) {
		t.Error("a folder this installer did not write was removed")
	}
	// Removing an installation that has no entries is not a failure.
	if actions := unlinkMacEntries([]string{macApplicationLink(home)}); len(actions) != 0 {
		t.Errorf("removing nothing reported %+v", actions)
	}

	// The same thing through the entry point `--uninstall` uses, with the home
	// directory redirected: the working link goes, the one holding somebody
	// else's folder stays, and neither platforms' entries are touched.
	if err := os.MkdirAll(filepath.Dir(macApplicationLink(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, macApplicationLink(home), "a link")
	entries := removeEntries("darwin")
	if len(entries) != 1 || entries[0].Path != macApplicationLink(home) {
		t.Errorf("the macOS uninstall reported %+v, want only the application link removed", entries)
	}
	if entries := removeEntries("windows"); entries != nil {
		t.Errorf("a Windows uninstall looked for macOS links: %+v", entries)
	}
}

// TestTheMacintoshInstallationSaysHowToReachTheCommand is the PATH half of a
// Unix installation: the installer writes no shell profile, so when ~/.local/bin
// is not on PATH it says the exact line to add - and says nothing when there is
// nothing to say.
func TestTheMacintoshInstallationSaysHowToReachTheCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	// A PATH that is somebody's own, and one directory in it.
	t.Setenv("PATH", filepath.Join(home, "existing")+string(os.PathListSeparator)+filepath.Join(home, "other"))

	if notice := pathNoticeFor("darwin"); notice != nil {
		t.Errorf("nothing is installed, and the tool still has a line to add: %+v", notice)
	}

	dir := commandDir(home)
	write(t, filepath.Join(dir, launcherBase), "MZ the launcher")
	notice := pathNoticeFor("darwin")
	if notice == nil {
		t.Fatal("an installed command outside PATH was not reported")
	}
	if notice.Dir != dir {
		t.Errorf("the notice names %q, want %q", notice.Dir, dir)
	}
	if want := `export PATH="` + dir + `:$PATH"`; notice.Line != want {
		t.Errorf("the line to add is %q, want %q", notice.Line, want)
	}

	// Once the directory is on PATH there is nothing left to add, and Windows
	// never has anything to add: it puts the directory there itself.
	t.Setenv("PATH", filepath.Join(home, "existing")+string(os.PathListSeparator)+dir)
	if notice := pathNoticeFor("darwin"); notice != nil {
		t.Errorf("a directory already on PATH was reported: %+v", notice)
	}
	if notice := pathNoticeFor("windows"); notice != nil {
		t.Errorf("Windows was told to edit a shell profile: %+v", notice)
	}
}

// TestTheMacintoshInstallationIsPerUser guards the promise decision 120 makes:
// nothing here asks for administrator rights, so nothing here writes outside the
// home directory - not the programs, not the entries.
func TestTheMacintoshInstallationIsPerUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	install, err := defaultInstallDir("darwin")
	if err != nil {
		t.Fatalf("defaultInstallDir: %v", err)
	}
	if want := filepath.Join(home, ".local", "lib", "theia"); install != want {
		t.Errorf("the macOS installation goes to %q, want %q", install, want)
	}
	if !within(install, home) {
		t.Errorf("%q is not under the home directory", install)
	}

	for _, link := range []string{macApplicationLink(home), macCommandLink(home)} {
		if !within(link, home) {
			t.Errorf("the entry %q is not under the home directory", link)
		}
		if within(link, install) {
			t.Errorf("the entry %q is inside the installation directory", link)
		}
	}
	if filepath.Base(macApplicationLink(home)) != darwinPlayerTree {
		t.Errorf("Finder would see %q", filepath.Base(macApplicationLink(home)))
	}
	if filepath.Base(macCommandLink(home)) != launcherBase {
		t.Errorf("a terminal would find %q", filepath.Base(macCommandLink(home)))
	}
}
