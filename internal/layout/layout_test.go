package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
)

// The macOS answer is the reason this package exists, so it is asserted here
// rather than only where it is used: the launcher that looked for a loose
// `theia-player` on a Mac passed every test it had, because every test it had
// ran on a platform where that is the right answer.
func TestAMacPlayerIsTheFileInsideTheBundle(t *testing.T) {
	if got, want := PlayerExecutable("darwin"), "Theia.app/Contents/MacOS/theia-player"; got != want {
		t.Errorf("the macOS player is %q, want %q", got, want)
	}
	if got, want := PlayerExecutable("windows"), "theia-player.exe"; got != want {
		t.Errorf("the Windows player is %q, want %q", got, want)
	}
	if got, want := PlayerExecutable("linux"), "theia-player"; got != want {
		t.Errorf("the Linux player is %q, want %q", got, want)
	}
}

func TestTheServerHasNoBundleOnAnyPlatform(t *testing.T) {
	if got, want := ServerExecutable("windows"), "theia-server.exe"; got != want {
		t.Errorf("the Windows server is %q, want %q", got, want)
	}
	for _, goos := range []string{"darwin", "linux", "freebsd"} {
		if got, want := ServerExecutable(goos), "theia-server"; got != want {
			t.Errorf("the %s server is %q, want %q", goos, got, want)
		}
	}
}

// The record is what an installation remembers for the programs inside it, and
// the order the answers are weighed in is the rule: somebody saying so for one
// run, then what the installation was made with, then the standard place.
func TestTheDataDirectoryComesFromTheRecordBesideThePrograms(t *testing.T) {
	installDir, dataDir := t.TempDir(), t.TempDir()
	if _, ok, err := ReadRecord(installDir); err != nil || ok {
		t.Fatalf("an installation with no record reported one: ok=%v err=%v", ok, err)
	}
	if err := WriteRecord(installDir, dataDir); err != nil {
		t.Fatal(err)
	}
	record, ok, err := ReadRecord(installDir)
	if err != nil || !ok {
		t.Fatalf("the record just written was not read back: ok=%v err=%v", ok, err)
	}
	if record.DataDir != dataDir {
		t.Errorf("the record says %q, want %q", record.DataDir, dataDir)
	}
	// With nobody saying otherwise, the record decides.
	t.Setenv(config.DataDirEnv, "")
	if got, err := DataDir(installDir); err != nil || got != dataDir {
		t.Errorf("DataDir = %q (%v), want the recorded %q", got, err, dataDir)
	}
	// The environment is an operator saying so for this run, and outranks it.
	env := t.TempDir()
	t.Setenv(config.DataDirEnv, env)
	if got, err := DataDir(installDir); err != nil || got != env {
		t.Errorf("DataDir = %q (%v), want the environment's %q", got, err, env)
	}
}

// An installation made before the record existed has none, and falls to the
// standard place rather than refusing to start.
func TestAnInstallationWithNoRecordFallsToTheStandardPlace(t *testing.T) {
	t.Setenv(config.DataDirEnv, "")
	want, err := config.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DataDir(t.TempDir()); err != nil || got != want {
		t.Errorf("DataDir = %q (%v), want the standard %q", got, err, want)
	}
}

// A record that exists and names nothing is an error rather than a quiet fall to
// the standard place: looking in the wrong directory and saying nothing is the
// fault the record was written to end.
func TestARecordThatNamesNothingIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, RecordFile), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadRecord(dir); err == nil {
		t.Error("a record with no directory in it was accepted")
	}
	t.Setenv(config.DataDirEnv, "")
	if _, err := DataDir(dir); err == nil {
		t.Error("DataDir accepted a record with no directory in it")
	}
}
