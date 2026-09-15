//go:build windows

package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// The shortcut is read back by PowerShell and WScript.Shell - the same shell
// component a launcher uses. Reading it with another Go function would only
// prove that the writer and the reader in this repository agree with each other,
// which is exactly the mistake the .lnk format punishes.
func TestWriteShortcutIsReadBackByTheWindowsShell(t *testing.T) {
	powershell := powershellPath(t)

	target, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "Theia - Server.lnk")

	// A quoted path with a space in it on purpose: it is the shape of every
	// install directory on Windows, and an argument that survives the round trip
	// unquoted would hide a bug the user meets on their first launch.
	const arguments = `--data-dir "C:\Theia Test\data" --port 8395`

	s := Shortcut{
		Path:        path,
		Target:      target,
		Arguments:   arguments,
		WorkingDir:  dir,
		Description: "Theia - regarder ses films",
		Icon:        target,
	}
	if err := WriteShortcut(s); err != nil {
		t.Fatalf("WriteShortcut: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no .lnk was written: %v", err)
	}

	read := readShortcut(t, powershell, path)
	for field, want := range map[string]string{
		"TargetPath":       target,
		"Arguments":        arguments,
		"WorkingDirectory": dir,
		"Description":      s.Description,
	} {
		if got := read[field]; got != want {
			t.Errorf("the shell reads %s as %q, want %q", field, got, want)
		}
	}
	// IconLocation comes back as "path,index", which is how the shell stores it.
	if got := read["IconLocation"]; got != target+",0" {
		t.Errorf("the shell reads IconLocation as %q, want %q", got, target+",0")
	}

	// And it can be replaced, which is what re-running the installer does. The
	// old file must not survive underneath the new one: a .lnk is a compound
	// file, and Save onto an existing one has to truncate it.
	s.Description = "second time"
	if err := WriteShortcut(s); err != nil {
		t.Fatalf("WriteShortcut over an existing .lnk: %v", err)
	}
	if got := readShortcut(t, powershell, path)["Description"]; got != s.Description {
		t.Errorf("the shell reads Description as %q after rewriting, want %q", got, s.Description)
	}
}

// The Start Menu folder does not exist until somebody opens the Start Menu, and
// IPersistFile::Save answers STG_E_PATHNOTFOUND rather than creating it. The
// installer is the thing that has to make the folder.
func TestWriteShortcutCreatesTheFolderItWritesInto(t *testing.T) {
	powershell := powershellPath(t)

	target, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	// Several levels, and none of them there.
	path := filepath.Join(t.TempDir(), "Programs", "Theia", "Theia Player.lnk")
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("the test folder already exists, so it proves nothing: %v", err)
	}

	if err := WriteShortcut(Shortcut{Path: path, Target: target}); err != nil {
		t.Fatalf("WriteShortcut into a folder that does not exist: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no .lnk was written: %v", err)
	}
	// Stat alone would be satisfied by an empty file: the shell has to be able to
	// open it as a shortcut.
	if got := readShortcut(t, powershell, path)["TargetPath"]; got != target {
		t.Errorf("the shell reads TargetPath as %q, want %q", got, target)
	}
}

// WriteShortcut asks for a single-threaded apartment, and CoInitializeEx answers
// RPC_E_CHANGED_MODE rather than failing when the thread is already in another
// one - which happens as soon as anything else in the process reaches COM first.
// The apartment is therefore forced to multi-threaded here, so the call under
// test has to cope with an answer it did not choose.
func TestWriteShortcutWorksFromAnApartmentItDidNotChoose(t *testing.T) {
	powershell := powershellPath(t)

	// An apartment belongs to the thread, so the thread is held still while it is
	// put into one - for the initialisation here and for the call under test.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err != nil {
		t.Skipf("this thread is already in another apartment (%v), so the changed-mode answer cannot be forced", err)
	}
	defer windows.CoUninitialize()

	target, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	path := filepath.Join(t.TempDir(), "Theia - from the MTA.lnk")
	if err := WriteShortcut(Shortcut{
		Path:      path,
		Target:    target,
		Arguments: "--from-mta",
	}); err != nil {
		t.Fatalf("WriteShortcut on a thread that is already multi-threaded: %v", err)
	}

	read := readShortcut(t, powershell, path)
	if got := read["TargetPath"]; got != target {
		t.Errorf("the shell reads TargetPath as %q, want %q", got, target)
	}
	if got := read["Arguments"]; got != "--from-mta" {
		t.Errorf("the shell reads Arguments as %q, want %q", got, "--from-mta")
	}
}

func TestShortcutStartMenuDirFollowsTheRedirectedAppData(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)

	dir, err := StartMenuDir()
	if err != nil {
		t.Fatalf("StartMenuDir: %v", err)
	}
	want := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs")
	if dir != want {
		t.Errorf("StartMenuDir is %q, want %q", dir, want)
	}

	// The redirected APPDATA is not just an assertion: WriteShortcut is expected
	// to create the folder, so this is where a real install would land.
	if err := WriteShortcut(Shortcut{
		Path:   filepath.Join(dir, "Theia.lnk"),
		Target: os.Args[0],
	}); err != nil {
		t.Fatalf("WriteShortcut into the Start Menu folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Theia.lnk")); err != nil {
		t.Errorf("the Start Menu entry is not there: %v", err)
	}
	if !strings.HasPrefix(dir, appData) {
		t.Errorf("StartMenuDir ignored the redirected APPDATA: %q", dir)
	}
}

// DesktopDir asks the shell, so the check is that the answer is a real folder on
// this machine - not that it equals a path this test could have guessed, which is
// what %USERPROFILE%\Desktop would be.
func TestShortcutDesktopDirNamesAFolderThatExists(t *testing.T) {
	desktop, err := DesktopDir()
	if err != nil {
		t.Fatalf("DesktopDir: %v", err)
	}
	if !filepath.IsAbs(desktop) {
		t.Errorf("DesktopDir returned %q, which is not an absolute path", desktop)
	}
	if _, err := os.Stat(desktop); err != nil {
		t.Errorf("the Desktop the shell reports is not a folder on this machine: %v", err)
	}
}

// readShortcut asks the Windows shell what a .lnk says and returns its fields by
// name. The path travels in the environment rather than inside the -Command
// text: a temporary directory can contain a quote or a backtick, and a
// single-quoted PowerShell literal built from it would fail - or worse, quietly
// read a different file than the one just written.
func readShortcut(t *testing.T, powershell, path string) map[string]string {
	t.Helper()

	const script = `$s = (New-Object -ComObject WScript.Shell).CreateShortcut($env:THEIA_TEST_LNK)
Write-Output ("TargetPath=" + $s.TargetPath)
Write-Output ("Arguments=" + $s.Arguments)
Write-Output ("WorkingDirectory=" + $s.WorkingDirectory)
Write-Output ("Description=" + $s.Description)
Write-Output ("IconLocation=" + $s.IconLocation)`

	cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "THEIA_TEST_LNK="+path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("powershell could not read %s: %v\n%s", path, err, out)
	}

	fields := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[name] = value
	}
	if len(fields) == 0 {
		t.Fatalf("powershell read nothing out of %s:\n%s", path, out)
	}
	return fields
}

// powershellPath skips only when there is genuinely no PowerShell to run: an
// absent one is the one case where this file cannot be verified, and pretending
// otherwise would turn a machine without it into a passing test.
func powershellPath(t *testing.T) string {
	t.Helper()
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return path
	}
	if root := os.Getenv("SystemRoot"); root != "" {
		path := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	t.Skip("powershell.exe is not on PATH or in %SystemRoot%\\System32\\WindowsPowerShell\\v1.0")
	return ""
}
