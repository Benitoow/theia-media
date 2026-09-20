package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The entries are what makes an installation findable. They are checked here on
// the policy - which names, which programs, where - and the .lnk format itself is
// checked by shortcut_windows_test.go, which reads one back with Windows.

func TestTheEntriesNameEveryProgramTheRoleInstalled(t *testing.T) {
	requireWindowsForEntries(t)
	french, _ := CatalogueFor("fr")
	install := t.TempDir()
	for _, name := range []string{installed("theia-server"), installed("theia-player"), "libmpv-2.dll"} {
		write(t, filepath.Join(install, name), "MZ")
	}

	targets := ShortcutTargets{StartMenu: t.TempDir(), Desktop: t.TempDir()}
	result, failure := createShortcuts(
		Plan{Role: RoleAllInOne, InstallDir: install}.WithDefaults(""), targets, french)
	if failure != "" {
		t.Fatalf("creating the entries failed: %s", failure)
	}

	// The Start Menu gets one entry per program, in one folder named after the
	// product - which is the folder a launcher indexes.
	folder := filepath.Join(targets.StartMenu, french["shortcutTheiaName"])
	wanted := []string{
		french["shortcutTheiaName"] + ".lnk",
		french["shortcutServerName"] + ".lnk",
		french["shortcutPlayerName"] + ".lnk",
	}
	for _, name := range wanted {
		if _, err := os.Stat(filepath.Join(folder, name)); err != nil {
			t.Errorf("the Start Menu has no %s: %v", name, err)
		}
	}
	// And the Desktop gets the product alone: three icons for one program is how
	// a Desktop stops being a place somebody put things.
	desktop, err := os.ReadDir(targets.Desktop)
	if err != nil {
		t.Fatal(err)
	}
	if len(desktop) != 1 || desktop[0].Name() != french["shortcutTheiaName"]+".lnk" {
		t.Errorf("the Desktop holds %d entries: %v", len(desktop), names(desktop))
	}
	if len(result) != 4 {
		t.Errorf("the installation reported %d entries, want three in the menu and one on the Desktop", len(result))
	}
	for _, action := range result {
		if action.Kind != "created-shortcut" || action.Path == "" {
			t.Errorf("an entry was reported as %+v", action)
		}
	}
}

func TestTheEntriesShowTheProductsMarkWithoutBorrowingOne(t *testing.T) {
	// Windows draws the target's own icon, and both Go binaries embed the
	// product's mark now (cmd/*/rsrc_windows_*.syso). Before that they carried
	// none, so the server's entries borrowed the player's - and a machine with
	// only the server had nothing to borrow, which is how three Theia entries
	// came to look like three unknown programs in a launcher.
	//
	// So no entry names an icon, and the assertion that it stays that way is the
	// resource object each Windows build links. Delete one and this fails, which
	// is the day the generic glyphs come back.
	french, _ := CatalogueFor("fr")
	install := t.TempDir()
	write(t, filepath.Join(install, installed("theia-server")), "MZ")
	write(t, filepath.Join(install, installed("theia-player")), "MZ")

	entries := entriesFor(Plan{Role: RoleAllInOne, InstallDir: install}, french)
	if len(entries) != 3 {
		t.Fatalf("the role produced %d entries, want 3", len(entries))
	}
	for _, entry := range entries {
		if entry.link.Icon != "" {
			t.Errorf("%s names an icon at %q; Windows already draws the target's own", entry.name, entry.link.Icon)
		}
	}

	for program, path := range map[string]string{
		"theia-server": filepath.Join("..", "..", "cmd", "theia-server", "rsrc_windows_amd64.syso"),
		"theia-setup":  filepath.Join("..", "..", "cmd", "theia-setup", "rsrc_windows_amd64.syso"),
		"theia-player": filepath.Join("..", "..", "player", "theia-player", "icons", "icon.ico"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s carries no icon resource: %v", program, err)
		}
	}
}

func TestTheProductEntryOpensTheViewerForAnAllInOne(t *testing.T) {
	french, _ := CatalogueFor("fr")
	install := t.TempDir()
	entries := entriesFor(Plan{Role: RoleAllInOne, InstallDir: install}, french)
	if len(entries) == 0 {
		t.Fatal("the all-in-one role produced no product entry")
	}
	want := filepath.Join(install, installed("theia-player"))
	if !strings.EqualFold(entries[0].link.Target, want) {
		t.Errorf("Theia opens %q, want the viewer %q", entries[0].link.Target, want)
	}
}

func TestAPlayerOnlyMachineGetsNoServerEntry(t *testing.T) {
	// An entry that starts a program this machine does not have is a broken
	// promise somebody double-clicks. The role decides, and the player-only role
	// must not leave a server behind it.
	requireWindowsForEntries(t)
	french, _ := CatalogueFor("fr")
	install := t.TempDir()
	write(t, filepath.Join(install, installed("theia-player")), "MZ")

	targets := ShortcutTargets{StartMenu: t.TempDir(), Desktop: t.TempDir()}
	if _, failure := createShortcuts(Plan{Role: RolePlayer, InstallDir: install}, targets, french); failure != "" {
		t.Fatalf("creating the entries failed: %s", failure)
	}

	entries, err := os.ReadDir(filepath.Join(targets.StartMenu, french["shortcutTheiaName"]))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("a player-only machine left %d entries: %v", len(entries), names(entries))
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "Server") {
			t.Errorf("a player-only machine was given a server entry: %s", entry.Name())
		}
	}
}

func TestAnEntryIsNotWrittenForAProgramThatIsNotThere(t *testing.T) {
	// The programs are installed before the entries, so a missing one means the
	// installation is not what it claims. It is reported, and no entry points at
	// a file that does not exist.
	requireWindowsForEntries(t)
	french, _ := CatalogueFor("fr")
	install := t.TempDir() // deliberately empty
	targets := ShortcutTargets{StartMenu: t.TempDir(), Desktop: t.TempDir()}

	actions, failure := createShortcuts(Plan{Role: RoleServer, InstallDir: install}, targets, french)
	if failure == "" {
		t.Fatal("a missing program produced no complaint")
	}
	if len(actions) != 0 {
		t.Errorf("an entry was created for a program that is not installed: %+v", actions)
	}
	if entries, err := os.ReadDir(filepath.Join(targets.StartMenu, french["shortcutTheiaName"])); err == nil && len(entries) != 0 {
		t.Errorf("the Start Menu folder was created with %d entries in it", len(entries))
	}
}

func TestTheDescriptionsAreSentencesInTheChosenLanguage(t *testing.T) {
	// A tooltip is read by a person, so it belongs to the catalogue. A French
	// installation must not leave an English tooltip behind, and the two
	// languages must not be the same string.
	french, _ := CatalogueFor("fr")
	english, _ := CatalogueFor("en")
	for _, key := range []string{"shortcutTheia", "shortcutServer", "shortcutPlayer"} {
		if french[key] == "" || english[key] == "" {
			t.Errorf("%s is empty in one of the languages", key)
		}
		if french[key] == english[key] {
			t.Errorf("%s is the same sentence in both languages: %q", key, french[key])
		}
	}
	// The names are proper nouns: the same word in both languages, and not the
	// brand alone, which is what somebody typing "Theia Server" would otherwise
	// have to guess.
	for _, key := range []string{"shortcutTheiaName", "shortcutServerName", "shortcutPlayerName"} {
		if french[key] != english[key] {
			t.Errorf("%s is translated (%q / %q) and should be a proper noun", key, french[key], english[key])
		}
	}
}

func TestEntriesAreOnlyWrittenOnWindows(t *testing.T) {
	// V3.3 is verified on Windows only. On another platform the programs are
	// installed and no claim is made about a menu nobody has run.
	if runtime.GOOS == "windows" {
		t.Skip("this is the Windows path; the guard is for the others")
	}
	french, _ := CatalogueFor("fr")
	install := t.TempDir()
	write(t, filepath.Join(install, "theia-server"), "ELF")
	targets := ShortcutTargets{StartMenu: t.TempDir(), Desktop: t.TempDir()}
	actions, failure := createShortcuts(Plan{Role: RoleServer, InstallDir: install}, targets, french)
	if len(actions) != 0 || failure != "" {
		t.Errorf("a non-Windows platform was given entries: %+v %q", actions, failure)
	}
}

// requireWindowsForEntries skips the entry tests that assert files on disk:
// createShortcuts is a deliberate no-op on every other platform, which is what
// TestEntriesAreOnlyWrittenOnWindows is the test for. The tests that only read
// what entriesFor builds run everywhere, because that rule has no platform in
// it beyond the name each program carries.
func requireWindowsForEntries(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("the entries are only written on Windows; TestEntriesAreOnlyWrittenOnWindows is the guard for the others")
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	return out
}
