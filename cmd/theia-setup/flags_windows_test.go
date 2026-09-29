//go:build windows

// This file is in `package main` rather than a `_test` package because a Go
// directory holds one package: the product here is the binary itself, so the
// guard builds and runs it rather than calling into it.
package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The command line, driven for real.
//
// Everything else in the setup package tests functions. `--from`, `--force` and
// `--uninstall` are not functions, they are *wiring*: a flag parsed in `main`,
// handed to a helper, and turned into a Source or a call. A wiring mistake is
// invisible to a library test - `--force` existed once and reached nothing, which
// is exactly the shape of bug that a test of `Source{Force: true}` cannot see.
//
// So this builds the real `theia-setup` and runs it as a subprocess, with the
// same redirected APPDATA, LOCALAPPDATA and USERPROFILE the other installer tests
// use.
//
// Filesystem redirection does not isolate HKCU. Link the subprocess with test
// names for every registry root too: otherwise even --uninstall with no files
// removes the real Windows Apps and App Paths entries. The production binary
// keeps the package defaults; these overrides exist only in this test build.
const isolatedRegistryFlags = "-X github.com/Benitoow/theia-media/internal/setup.registeredName=Theia-cli-tests " +
	"-X github.com/Benitoow/theia-media/internal/setup.userPathKey=Software\\Theia\\CLItests\\Environment " +
	"-X github.com/Benitoow/theia-media/internal/setup.appPathsKey=Software\\Theia\\CLItests\\AppPaths " +
	"-X github.com/Benitoow/theia-media/internal/setup.taskName=Theia-cli-tests"

// buildInstaller compiles cmd/theia-setup once for this test file.
//
// A build failure skips rather than fails: a toolchain that is absent is not a
// fault in the installer, and a guard that cannot run should say so instead of
// reporting red about the wrong thing.
func buildInstaller(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "theia-setup.exe")

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", isolatedRegistryFlags, "-o", out, "./cmd/theia-setup")
	cmd.Dir = filepath.Join("..", "..")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build the installer (no Go toolchain?): %v\n%s", err, output)
	}
	return out
}

// isolatedEnv is the environment one run sees.
//
// `SystemRoot` is kept because Windows APIs read it and a child without it fails
// in ways that look like faults in the program. Everything the installer writes
// to is replaced by a directory this test owns, so the machine's Start Menu, its
// Desktop and its logon folder are never in reach.
func isolatedEnv(t *testing.T, root string) (env []string, appData, localAppData, profile string) {
	t.Helper()
	appData = filepath.Join(root, "appdata")
	localAppData = filepath.Join(root, "localappdata")
	profile = filepath.Join(root, "profile")
	for _, dir := range []string{appData, localAppData, profile} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	kept := map[string]bool{
		"SystemRoot": true, "windir": true, "PATH": true, "PATHEXT": true,
		"TEMP": true, "TMP": true, "COMSPEC": true, "SystemDrive": true,
		"NUMBER_OF_PROCESSORS": true,
	}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if kept[name] {
			env = append(env, entry)
		}
	}
	return append(env,
		"APPDATA="+appData,
		"LOCALAPPDATA="+localAppData,
		"USERPROFILE="+profile,
		"HOME="+profile,
	), appData, localAppData, profile
}

// runSetup runs the built installer and returns its exit code and combined output.
func runSetup(t *testing.T, exe, root string, env []string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = root
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("running %v: %v", args, err)
	}
	return exit.ExitCode(), string(output)
}

// countUnder lists everything under a directory.
func countUnder(t *testing.T, dir string) []string {
	t.Helper()
	var found []string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}
		found = append(found, path)
		return nil
	})
	return found
}

// TestTheServiceFlagWithoutYesFailsBeforeItWrites is a refusal, and a refusal is
// exactly what a subprocess test can settle.
//
// `--service` installs an autostart entry, which is a change to somebody's
// machine, so the scripted path asks once and `--yes` is how a script answers.
// The check sits before the plan is installed, so nothing is written - and that
// ordering is the whole reason this test is safe to run as a subprocess.
func TestTheServiceFlagWithoutYesFailsBeforeItWrites(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the Windows entries are the ones being kept away from")
	}
	exe := buildInstaller(t)
	root := t.TempDir()
	env, appData, _, _ := isolatedEnv(t, root)

	installDir := filepath.Join(root, "programs")
	dataDir := filepath.Join(root, "data")
	source := filepath.Join(root, "release")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "theia-server.exe"), []byte("MZ the server"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, output := runSetup(t, exe, root, env,
		"--role", "server", "--service", "--from", source,
		"--install-dir", installDir, "--data-dir", dataDir, "--lang", "fr")

	if code == 0 {
		t.Error("--service without --yes was accepted, which installs an autostart entry nobody confirmed")
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Errorf("the refusal still created %s (stat error %v)", installDir, err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Errorf("the refusal still created %s (stat error %v)", dataDir, err)
	}
	if found := countUnder(t, appData); len(found) != 0 {
		t.Errorf("the refusal wrote %d paths under APPDATA, first %s", len(found), found[0])
	}
	if !strings.Contains(output, "--yes") {
		t.Errorf("the refusal does not say what is missing; it printed:\n%s", output)
	}
}

// TestUninstallOnAMachineWithNothingInstalledSucceedsAndKeepsTheLibrary drives
// the one flag that must never need a form.
//
// This is the command the applications list runs. It has to work with nobody
// watching and nothing installed, and the installation directory is redirected to
// a folder that was never created, so the programs it removes are none. The
// sentence that matters is the one about the data directory: somebody removing a
// program wants to be sure their library did not go with it.
func TestUninstallOnAMachineWithNothingInstalledSucceedsAndKeepsTheLibrary(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the Windows entries are the ones being kept away from")
	}
	exe := buildInstaller(t)
	root := t.TempDir()
	env, _, localAppData, _ := isolatedEnv(t, root)

	// Something that must survive, planted where the installer would have put it.
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(dataDir, "theia.db")
	if err := os.WriteFile(history, []byte("somebody's watch history"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, output := runSetup(t, exe, root, env, "--uninstall", "--lang", "fr")
	if code != 0 {
		t.Fatalf("--uninstall with nothing installed exited %d:\n%s", code, output)
	}
	if _, err := os.Stat(history); err != nil {
		t.Errorf("the watch history did not survive the uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(localAppData, "Programs", "Theia")); !os.IsNotExist(err) {
		t.Errorf("the uninstall created an installation directory (stat error %v)", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Error("the uninstall printed nothing at all")
	}
}

func TestInstalledUninstallerRemovesItsCustomDirectoryAndKeepsDefaultInstallation(t *testing.T) {
	exe := buildInstaller(t)
	root := t.TempDir()
	env, _, local, _ := isolatedEnv(t, root)
	custom := filepath.Join(root, "custom installation")
	data := filepath.Join(root, "data")
	standard := filepath.Join(local, "Programs", "Theia")
	for _, dir := range []string{custom, data, standard} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installed := filepath.Join(custom, "theia-setup.exe")
	bytes, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, bytes, 0o755); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(map[string]string{"data_dir": data})
	if err := os.WriteFile(filepath.Join(custom, "installation.json"), record, 0o644); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(data, "theia.db")
	sentinel := filepath.Join(standard, "keep.txt")
	for _, path := range []string{history, sentinel} {
		if err := os.WriteFile(path, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, output := runSetup(t, installed, root, env, "--uninstall", "--json")
	if code != 0 {
		t.Fatalf("uninstall exited %d: %s", code, output)
	}
	// Windows releases the running setup after exit; its hidden removal helper
	// must finish the custom directory without touching the standard one.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(custom); os.IsNotExist(err) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatalf("custom installation remains: %v", err)
	}
	for _, path := range []string{history, sentinel} {
		if bytes, err := os.ReadFile(path); err != nil || string(bytes) != "keep" {
			t.Fatalf("uninstall changed %s: %v", path, err)
		}
	}
}

// TestCheckChangesNothing is the read-only promise, measured the only way it can
// be: by looking at the machine before and after.
//
// `--check` is what somebody runs to find out what this machine is, and a
// diagnostic that quietly writes is worse than no diagnostic. It is also, for the
// same reason as the two above, safe to drive as a subprocess.
func TestCheckChangesNothing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the Windows entries are the ones being kept away from")
	}
	exe := buildInstaller(t)
	root := t.TempDir()
	env, appData, localAppData, profile := isolatedEnv(t, root)

	before := len(countUnder(t, appData)) + len(countUnder(t, localAppData)) + len(countUnder(t, profile))

	for _, args := range [][]string{
		{"--check"},
		{"--check", "--json"},
		{"--check", "--lang", "en"},
	} {
		code, output := runSetup(t, exe, root, env, args...)
		if code != 0 {
			t.Errorf("%v exited %d:\n%s", args, code, output)
			continue
		}
		if len(args) == 2 && args[1] == "--json" {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Errorf("--check --json did not print JSON: %v\n%s", err, output)
			}
		}
	}

	after := len(countUnder(t, appData)) + len(countUnder(t, localAppData)) + len(countUnder(t, profile))
	if after != before {
		t.Errorf("--check wrote %d paths (%d before, %d after)", after-before, before, after)
	}
}
