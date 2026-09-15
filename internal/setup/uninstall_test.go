package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Uninstalling is the one operation that can lose somebody's library, so what is
// checked here is mostly what it does *not* remove.

// fakeInstallation builds what the installer leaves: programs in one folder,
// entries in a Start Menu folder and on the Desktop, a data directory, and a
// registered application.
func fakeInstallation(t *testing.T, text Catalogue) (Plan, ShortcutTargets) {
	t.Helper()

	// The autostart entry lives under APPDATA, and the tests must never touch
	// the folder Windows actually reads at logon: redirected, exactly as the
	// service tests do it.
	t.Setenv("APPDATA", t.TempDir())

	// The applications-list entry is redirected by TestMain, for every test in
	// the package: there is no APPDATA to redirect in the registry, and a test
	// that used the real name would unregister the machine it runs on.

	install := t.TempDir()
	write(t, filepath.Join(install, "theia-server.exe"), "MZ the server")
	write(t, filepath.Join(install, "theia-player.exe"), "MZ the player")
	write(t, filepath.Join(install, "libmpv-2.dll"), "engine")
	write(t, filepath.Join(install, "libmpv-2.dll.previous.1"), "an old engine")
	write(t, filepath.Join(install, installerExecutable("windows")), "MZ the tool")

	targets := ShortcutTargets{StartMenu: t.TempDir(), Desktop: t.TempDir()}
	folder := filepath.Join(targets.StartMenu, text["shortcutTheiaName"])
	for _, name := range shortcutNames(text) {
		write(t, filepath.Join(folder, name+".lnk"), "a shortcut")
	}
	write(t, filepath.Join(targets.Desktop, text["shortcutTheiaName"]+".lnk"), "a shortcut")

	plan := Plan{
		Role:       RoleAllInOne,
		DataDir:    t.TempDir(),
		InstallDir: install,
		Port:       8395,
		Hostname:   "theia",
	}
	write(t, filepath.Join(plan.DataDir, "theia.db"), "a library")
	write(t, filepath.Join(plan.DataDir, "config.json"), "{}")
	return plan, targets
}

func TestUninstallingTakesTheProgramsAndTheEntries(t *testing.T) {
	text, _ := CatalogueFor("fr")
	plan, targets := fakeInstallation(t, text)
	if err := registerApplication(applicationKeyPath(registeredName), Application{
		Name: "Theia", Version: "3.3.0", InstallLocation: plan.InstallDir,
	}); err != nil {
		t.Fatalf("registerApplication: %v", err)
	}

	result, err := Uninstall(plan, targets, text)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if _, err := os.Stat(plan.InstallDir); !os.IsNotExist(err) {
		t.Errorf("the programs are still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targets.StartMenu, text["shortcutTheiaName"])); !os.IsNotExist(err) {
		t.Errorf("the Start Menu folder is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targets.Desktop, text["shortcutTheiaName"]+".lnk")); !os.IsNotExist(err) {
		t.Errorf("the Desktop entry is still there: %v", err)
	}
	if applicationIsRegistered(applicationKeyPath(registeredName)) {
		t.Error("the application is still in the installed programs list")
	}
	unregisterApplication(applicationKeyPath(registeredName))
	_ = 0

	// The actions say what happened, as codes: the interface owns the sentence.
	kinds := map[string]int{}
	for _, action := range result.Actions {
		kinds[action.Kind]++
	}
	for _, want := range []string{"removed-program", "removed-shortcut", "unregistered-application"} {
		if kinds[want] == 0 {
			t.Errorf("no %q action was reported: %+v", want, result.Actions)
		}
	}
	if kinds["removed-shortcut"] != 4 {
		t.Errorf("%d shortcuts were reported removed, want 4", kinds["removed-shortcut"])
	}
}

func TestUninstallingKeepsTheLibrary(t *testing.T) {
	// The whole point. A program that removes somebody's watch history while
	// "removing itself" is a program nobody forgives, and the data directory
	// holds the library, the progress marks and the configuration.
	text, _ := CatalogueFor("fr")
	plan, targets := fakeInstallation(t, text)

	result, err := Uninstall(plan, targets, text)
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	for _, name := range []string{"theia.db", "config.json"} {
		path := filepath.Join(plan.DataDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s did not survive the uninstall: %v", name, err)
		}
	}
	if result.DataDir != plan.DataDir {
		t.Errorf("the result names the data directory as %q, want %q", result.DataDir, plan.DataDir)
	}
	if !strings.Contains(strings.Join(actionPaths(result), " "), plan.DataDir) {
		t.Errorf("the uninstall never mentioned where the data is: %+v", result.Actions)
	}
}

func TestUninstallingSomethingThatIsNotThereIsNotAFailure(t *testing.T) {
	// The command is registered with Windows and can be run twice, or run on a
	// machine where somebody deleted the folder by hand. Neither is an error.
	text, _ := CatalogueFor("fr")
	t.Setenv("APPDATA", t.TempDir())
	plan := Plan{
		Role:       RoleServer,
		DataDir:    t.TempDir(),
		InstallDir: filepath.Join(t.TempDir(), "never-installed"),
	}
	targets := ShortcutTargets{StartMenu: t.TempDir(), Desktop: t.TempDir()}

	result, err := Uninstall(plan, targets, text)
	if err != nil {
		t.Fatalf("Uninstall on nothing: %v", err)
	}
	notInstalled := false
	for _, action := range result.Actions {
		if action.Kind == "nothing-installed" {
			notInstalled = true
		}
	}
	if !notInstalled {
		t.Errorf("the result does not say there was nothing to remove: %+v", result.Actions)
	}
}

func TestAnEntrySomebodyElsePutInTheFolderIsLeftAlone(t *testing.T) {
	// The Start Menu folder is the product's, but a shortcut in it that this
	// installer did not write is not: removing it would delete something a
	// person put there.
	text, _ := CatalogueFor("fr")
	plan, targets := fakeInstallation(t, text)
	stranger := filepath.Join(targets.StartMenu, text["shortcutTheiaName"], "Somebody Else.lnk")
	write(t, stranger, "not ours")

	if _, err := Uninstall(plan, targets, text); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(stranger); err != nil {
		t.Errorf("a shortcut this installer did not write was deleted: %v", err)
	}
	// And our entries are gone from that folder.
	for _, name := range shortcutNames(text) {
		path := filepath.Join(targets.StartMenu, text["shortcutTheiaName"], name+".lnk")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s was not removed: %v", filepath.Base(path), err)
		}
	}
}

func actionPaths(result Result) []string {
	paths := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		paths = append(paths, action.Path)
	}
	return paths
}
