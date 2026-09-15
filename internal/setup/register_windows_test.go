//go:build windows

package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// The applications list is what Windows shows under "Programs and Features" and
// what a launcher's registry source reads. These tests write under a key they own
// - the registry has no APPDATA to redirect - and delete it afterwards, because a
// test that touched the real entry would change the machine it runs on.

// testKeyPath returns a path under Software\Theia\tests and removes the tree when
// the test ends, whatever happened.
func testKeyPath(t *testing.T, name string) string {
	t.Helper()
	path := fmt.Sprintf(`Software\Theia\tests\%s\%s`, strings.ReplaceAll(t.Name(), "/", "-"), name)
	t.Cleanup(func() {
		registry.DeleteKey(registry.CURRENT_USER, path)
		// The two parents are ours: created by this test, empty when it is done.
		parent := strings.TrimSuffix(path, `\`+name)
		registry.DeleteKey(registry.CURRENT_USER, parent)
		registry.DeleteKey(registry.CURRENT_USER, `Software\Theia\tests`)
		registry.DeleteKey(registry.CURRENT_USER, `Software\Theia`)
	})
	return path
}

func TestTheApplicationIsRegisteredWhereWindowsListsIt(t *testing.T) {
	install := t.TempDir()
	write(t, filepath.Join(install, "theia-player.exe"), strings.Repeat("MZ", 2048)) // 4 KB
	write(t, filepath.Join(install, "libmpv-2.dll"), strings.Repeat("MZ", 4096))

	plan := Plan{Role: RoleAllInOne, InstallDir: install}
	uninstaller := filepath.Join(install, "theia-setup.exe")
	app := defaultApplication(plan, "3.3.0", uninstaller)

	// The icon is the program with a window, not the server: it is what a person
	// sees beside the name.
	if !strings.EqualFold(app.Icon, filepath.Join(install, "theia-player.exe")) {
		t.Errorf("the icon points at %q", app.Icon)
	}
	if app.SizeKB != 12 {
		t.Errorf("the recorded size is %d KB, want 12", app.SizeKB)
	}

	keyPath := testKeyPath(t, applicationKeyName)
	if err := registerApplication(keyPath, app); err != nil {
		t.Fatalf("registerApplication: %v", err)
	}
	if !applicationIsRegistered(keyPath) {
		t.Fatal("the entry was written and is not there")
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()

	for name, want := range map[string]string{
		"DisplayName":     "Theia",
		"DisplayVersion":  "3.3.0",
		"InstallLocation": install,
	} {
		got, _, err := key.GetStringValue(name)
		if err != nil {
			t.Errorf("%s is missing: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	remove, _, err := key.GetStringValue("UninstallString")
	if err != nil {
		t.Fatalf("UninstallString is missing: %v", err)
	}
	if !strings.Contains(remove, uninstaller) || !strings.HasSuffix(remove, UninstallFlag) {
		t.Errorf("UninstallString = %q, want it to name %s and end with %s", remove, uninstaller, UninstallFlag)
	}
	if !strings.HasPrefix(remove, `"`) {
		t.Errorf("UninstallString = %q: an installation folder with a space in it would break", remove)
	}
	if quiet, _, err := key.GetStringValue("QuietUninstallString"); err != nil || quiet != remove {
		t.Errorf("QuietUninstallString = %q (%v), want the same command", quiet, err)
	}
	for _, name := range []string{"NoModify", "NoRepair"} {
		value, _, err := key.GetIntegerValue(name)
		if err != nil || value != 1 {
			t.Errorf("%s = %d (%v), want 1: the installer has no such mode", name, value, err)
		}
	}

	if err := unregisterApplication(keyPath); err != nil {
		t.Fatalf("unregisterApplication: %v", err)
	}
	if applicationIsRegistered(keyPath) {
		t.Error("the entry is still in the list after being removed")
	}
	// Removing it twice is what a person does when they are not sure.
	if err := unregisterApplication(keyPath); err != nil {
		t.Errorf("removing an absent entry failed: %v", err)
	}
}

func TestAnApplicationsEntryNeverNeedsAdministratorRights(t *testing.T) {
	// Decision 120: this installer asks for no elevation. HKEY_CURRENT_USER is
	// the whole reason that is possible, so the key path is asserted rather than
	// assumed - a stray HKEY_LOCAL_MACHINE would fail on a locked-down machine
	// and would list one user's installation for everybody.
	path := applicationKeyPath("Theia")
	if !strings.HasPrefix(path, `Software\Microsoft\Windows\CurrentVersion\Uninstall`) {
		t.Errorf("the key is not where Windows looks: %q", path)
	}
	if strings.Contains(strings.ToUpper(path), "HKEY_LOCAL_MACHINE") || strings.Contains(path, "HKLM") {
		t.Errorf("the key needs administrator rights: %q", path)
	}
}

func TestTheFolderIsRemovedAfterTheProgramExits(t *testing.T) {
	// The uninstall normally runs from the copy inside the installation, and
	// Windows will not delete a running executable. The helper that outlives this
	// process is what finishes the job, so it is driven here rather than assumed:
	// the first version of this scheduled the deletion for the next start, which
	// needs administrator rights this installer never asks for, and the real
	// uninstall reported that some files could not be removed.
	// A space in the path on purpose: a profile folder is "C:\Users\John Doe" often
	// enough, and a command built by string concatenation breaks exactly there.
	dir := filepath.Join(t.TempDir(), "an installation")
	write(t, filepath.Join(dir, "theia-setup.exe"), "MZ pretending to be the running tool")

	if err := removeAfterExit(dir); err != nil {
		t.Fatalf("removeAfterExit: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Errorf("%s is still there twenty seconds after the helper was started", dir)
}

func TestTheRecordedSizeIsWhatIsOnDisk(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "one.bin"), strings.Repeat("x", 1024))
	write(t, filepath.Join(dir, "nested", "two.bin"), strings.Repeat("x", 3072))
	if got := directorySizeKB(dir); got != 4 {
		t.Errorf("directorySizeKB = %d, want 4", got)
	}
	if got := directorySizeKB(filepath.Join(dir, "absent")); got != 0 {
		t.Errorf("a folder that is not there reported %d KB", got)
	}
	_ = os.Remove(dir)
}
