//go:build windows

package setup

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// The PATH and App Paths entries, on the keys TestMain redirects into
// Software\Theia\tests: nothing here can reach the machine's real PATH or the
// real Run dialog.
//
// What is asserted is the arithmetic on the value and the type it is stored as.
// The restore has to be exact - a PATH is somebody's own list, and an installer
// that normalises it is an installer that quietly edits something it was only
// allowed to add one entry to.

func writePath(t *testing.T, value string, kind uint32) {
	t.Helper()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, userPathKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if err := setPathValue(key, value, kind); err != nil {
		t.Fatal(err)
	}
}

func readPath(t *testing.T) (string, uint32) {
	t.Helper()
	value, kind, err := userPath()
	if err != nil {
		t.Fatal(err)
	}
	return value, kind
}

func TestTheDirectoryIsAddedOnceAndTakenBackOut(t *testing.T) {
	writePath(t, `C:\Windows\system32`, registry.EXPAND_SZ)
	dir := `C:\Users\someone\AppData\Local\Programs\Theia`

	added, err := addToUserPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("the first installation reported that it added nothing")
	}
	if value, _ := readPath(t); value != `C:\Windows\system32;`+dir {
		t.Errorf("PATH is %q", value)
	}

	// A second run changes nothing: a PATH that has grown a second copy of the
	// same directory is a PATH somebody has to clean by hand.
	added, err = addToUserPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("a second installation added the directory again")
	}
	if value, _ := readPath(t); value != `C:\Windows\system32;`+dir {
		t.Errorf("PATH is %q after the second installation", value)
	}

	removed, err := removeFromUserPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("the uninstall reported that it removed nothing")
	}
	if value, _ := readPath(t); value != `C:\Windows\system32` {
		t.Errorf("PATH is %q after the uninstall", value)
	}
}

func TestSomebodysPathIsWrittenBackExactlyAsItWas(t *testing.T) {
	// The awkward parts of a real PATH: a trailing separator, a stray empty
	// entry, spaces, and an unexpanded variable. None of them is this
	// installer's business.
	const original = `C:\Program Files\Common;C:\Tools\;;%USERPROFILE%\bin`
	writePath(t, original, registry.EXPAND_SZ)
	dir := `C:\Users\someone\Programs\Theia`

	if _, err := addToUserPath(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := removeFromUserPath(dir); err != nil {
		t.Fatal(err)
	}
	if value, _ := readPath(t); value != original {
		t.Errorf("PATH came back as %q, want %q", value, original)
	}
}

func TestADirectoryIsRecognisedWhateverItsSeparatorOrCase(t *testing.T) {
	writePath(t, `C:\Tools\`, registry.SZ)
	if _, err := addToUserPath(`c:\tools`); err != nil {
		t.Fatal(err)
	}
	if value, _ := readPath(t); value != `C:\Tools\` {
		t.Errorf("PATH is %q: the same directory was added a second time", value)
	}
}

func TestTheTypeOfTheValueIsNotChanged(t *testing.T) {
	// %USERPROFILE% inside somebody's PATH only expands while the value is
	// expandable; writing it back as a plain string would quietly break it.
	for name, kind := range map[string]uint32{"expandable": registry.EXPAND_SZ, "plain": registry.SZ} {
		writePath(t, `C:\Windows`, kind)
		if _, err := addToUserPath(`C:\Users\someone\Programs\Theia`); err != nil {
			t.Fatal(err)
		}
		if _, got := readPath(t); got != kind {
			t.Errorf("a %s PATH came back as type %d, want %d", name, got, kind)
		}
	}
}

func TestRemovingTheLastEntryRemovesTheValue(t *testing.T) {
	// An empty PATH is not a PATH: a fresh Windows account has no Environment
	// value at all, and that is the state the uninstall puts things back into.
	dir := `C:\Users\someone\Programs\Theia`
	writePath(t, `C:\Windows`, registry.EXPAND_SZ)
	if _, err := addToUserPath(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := removeFromUserPath(dir); err != nil {
		t.Fatal(err)
	}
	// The directory that was there before is still there.
	if value, _ := readPath(t); value != `C:\Windows` {
		t.Fatalf("PATH is %q after removing the installation", value)
	}
	// And taking that one away too leaves no value behind at all.
	if _, err := removeFromUserPath(`C:\Windows`); err != nil {
		t.Fatal(err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, userPathKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if _, _, err := key.GetStringValue(userPathValue); err == nil {
		t.Error("the PATH value is still there after the last entry was removed")
	}
}

func TestTheWordTheiaAnswersWithTheInstalledCommand(t *testing.T) {
	const target = `C:\Users\someone\AppData\Local\Programs\Theia\theia.exe`
	t.Cleanup(func() { unregisterAppPath("theia.exe") })

	if err := registerAppPath("theia.exe", target); err != nil {
		t.Fatal(err)
	}
	if !appPathIsRegistered("theia.exe") {
		t.Fatal("the command was not registered")
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, appPathsKey+`\theia.exe`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	command, _, err := key.GetStringValue("")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(command, target) {
		t.Errorf("the Run dialog would start %q, want %q", command, target)
	}
	directory, _, err := key.GetStringValue("Path")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(directory, `C:\Users\someone\AppData\Local\Programs\Theia`) {
		t.Errorf("the working directory is %q", directory)
	}

	if err := unregisterAppPath("theia.exe"); err != nil {
		t.Fatal(err)
	}
	if appPathIsRegistered("theia.exe") {
		t.Error("the command is still registered after removal")
	}
	// And removing something that is not there is not a failure: this command
	// can be run twice, or on a machine where somebody deleted the key.
	if err := unregisterAppPath("theia.exe"); err != nil {
		t.Errorf("removing a command twice failed: %v", err)
	}
}

func TestUninstallingTakesTheNamesOffTheMachine(t *testing.T) {
	// A PATH entry pointing at a folder that is about to be deleted is a command
	// that exists and fails, and the App Paths entry is the same promise in the
	// Run dialog. Both go with the programs, and both are reported as actions so
	// the terminal can say so in the user's own language.
	text, _ := CatalogueFor("fr")
	plan, targets := fakeInstallation(t, text)
	launcher := filepath.Join(plan.InstallDir, "theia.exe")

	writePath(t, `C:\Windows\system32`, registry.EXPAND_SZ)
	if _, err := addToUserPath(plan.InstallDir); err != nil {
		t.Fatal(err)
	}
	if err := registerAppPath("theia.exe", launcher); err != nil {
		t.Fatal(err)
	}

	result, err := Uninstall(plan, targets, text)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if value, _ := readPath(t); strings.Contains(strings.ToLower(value), strings.ToLower(plan.InstallDir)) {
		t.Errorf("the installation directory is still on PATH: %q", value)
	}
	if appPathIsRegistered("theia.exe") {
		t.Error("the theia command is still registered with Windows")
	}

	kinds := map[string]int{}
	for _, action := range result.Actions {
		kinds[action.Kind]++
	}
	for _, want := range []string{"removed-from-path", "unregistered-app-path"} {
		if kinds[want] == 0 {
			t.Errorf("no %q action was reported: %+v", want, result.Actions)
		}
	}
}
