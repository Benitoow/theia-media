//go:build windows

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The autostart entry is written into a redirected APPDATA, so this exercises
// the real code path - the real folder layout, the real content, the real
// removal - without leaving a server on somebody's logon. The one thing it
// cannot verify is that Windows runs the entry at logon, which needs a logoff;
// that is reported as unverified rather than assumed.

// TestTheEntryStartsTheInstalledServer is about which of two real files the
// entry names. Somebody who runs the installer out of their Downloads folder has
// a server there and a server in the installation, and the one that starts at
// logon has to be the installation's: the download folder is the thing they will
// delete.
func TestTheEntryStartsTheInstalledServer(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	installed := filepath.Join(local, "Programs", "Theia")
	if err := os.MkdirAll(installed, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(installed, "theia-server.exe")
	if err := os.WriteFile(want, []byte("pretend"), 0o755); err != nil {
		t.Fatal(err)
	}

	// And a second copy beside this test binary, which is where a downloaded
	// archive leaves one. It must lose.
	beside := filepath.Join(filepath.Dir(mustExecutable(t)), "theia-server.exe")

	path, err := serverExecutable()
	if err != nil {
		t.Fatalf("serverExecutable: %v", err)
	}
	if !strings.EqualFold(path, want) {
		t.Errorf("the entry would start %s, want the installed %s (beside the binary: %s)", path, want, beside)
	}
}

func mustExecutable(t *testing.T) string {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheStartupEntryIsWrittenWhereWindowsLooksForIt(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)

	// A server binary beside the fake installer, so the entry has something real
	// to point at.
	installerDir := t.TempDir()
	server := filepath.Join(installerDir, "theia-server.exe")
	if err := os.WriteFile(server, []byte("pretend"), 0o755); err != nil {
		t.Fatal(err)
	}

	plan := Plan{
		Role:     RoleServer,
		DataDir:  filepath.Join(t.TempDir(), "a folder with spaces"),
		Port:     8383,
		Hostname: "theia",
	}
	kind, err := installAutostart(plan, server)
	if err != nil {
		t.Fatalf("installAutostart: %v", err)
	}

	// Not elevated in a test run, so the mechanism has to be the one that needs
	// no rights. If this ever reports a scheduled task, the test is running
	// elevated and the assertion below would be testing the wrong branch.
	if isElevated() {
		t.Skip("this shell is elevated, so the scheduled-task branch was taken instead")
	}
	if kind != autostartStartupEntry {
		t.Fatalf("installed %q without elevation, want %q", kind, autostartStartupEntry)
	}

	path, err := startupEntryPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, appData) {
		t.Errorf("the entry was written to %s, outside the redirected APPDATA", path)
	}
	if filepath.Base(filepath.Dir(path)) != "Startup" {
		t.Errorf("the entry is in %s, not a Startup folder", filepath.Dir(path))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the entry is not there after installing it: %v", err)
	}
	text := string(content)
	for _, want := range []string{server, "--data-dir", plan.DataDir, "/min"} {
		if !strings.Contains(text, want) {
			t.Errorf("the entry does not mention %q:\n%s", want, text)
		}
	}
	// The data directory has spaces in it on purpose: an unquoted path there
	// would start the server with half an argument.
	if !strings.Contains(text, `"`+plan.DataDir+`"`) {
		t.Errorf("the data directory is not quoted:\n%s", text)
	}

	// And it can be taken back off. A machine that cannot be un-installed is
	// worse than one that cannot be installed.
	removed, err := RemoveAutostart()
	if err != nil {
		t.Fatalf("RemoveAutostart: %v", err)
	}
	if removed.Kind != autostartStartupEntry {
		t.Errorf("removed %q, want %q", removed.Kind, autostartStartupEntry)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the entry is still there after removing it: %v", err)
	}
}

func TestStatusReportsTheEntryThatExists(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if status := autostartStatus(); status.Installed {
		t.Errorf("a machine with no entry reported %+v", status)
	}

	installerDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(installerDir, "theia-server.exe"), []byte("pretend"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Role: RoleServer, DataDir: t.TempDir()}
	if _, err := installAutostart(plan, filepath.Join(installerDir, "theia-server.exe")); err != nil && !isElevated() {
		t.Fatalf("installAutostart: %v", err)
	}
	status := autostartStatus()
	if !status.Installed {
		t.Fatalf("an installed entry was not reported: %+v", status)
	}
	// The note is what tells somebody why they got a startup entry instead of the
	// scheduled task the spec names. Without it the difference is invisible.
	if !isElevated() && status.Note == "" {
		t.Error("the status does not explain why a startup entry was used instead of a task")
	}
	if _, err := RemoveAutostart(); err != nil {
		t.Fatal(err)
	}
}
