package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// The names a binary has on disk are two, and both are real: the short one a
// working tree and `build.ps1` produce, and the platform-qualified one the
// release publishes. Looking for the short one only is how an installation with
// every published file sitting in one folder reported the server as missing and
// could not install its autostart entry.
func TestTheInstallerAcceptsTheNamesTheReleasePublishes(t *testing.T) {
	extension := ""
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}
	names := artifactNames("theia-server")
	if !slices.Contains(names, "theia-server"+extension) {
		t.Errorf("artifactNames = %q, want the name a working tree builds", names)
	}
	published := fmt.Sprintf("theia-server-%s-%s%s", runtime.GOOS, runtime.GOARCH, extension)
	if !slices.Contains(names, published) {
		t.Errorf("artifactNames = %q, want the name the release publishes (%s)", names, published)
	}
}

func TestAnInstallationFindsThePublishedFileBesideTheInstaller(t *testing.T) {
	dir := t.TempDir()
	extension := ""
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}
	published := filepath.Join(dir, fmt.Sprintf("theia-server-%s-%s%s", runtime.GOOS, runtime.GOARCH, extension))
	if err := os.WriteFile(published, []byte("pretend"), 0o755); err != nil {
		t.Fatal(err)
	}
	installer := filepath.Join(dir, "theia-setup"+extension)

	found, err := besideInstallerRelative(installer, artifactNames("theia-server"))
	if err != nil {
		t.Fatalf("the published file beside the installer was not found: %v", err)
	}
	if found != published {
		t.Errorf("found %q, want %q", found, published)
	}

	// And the plain name wins when both are there, because that is what a
	// development tree has and what the archive ships.
	plain := filepath.Join(dir, "theia-server"+extension)
	if err := os.WriteFile(plain, []byte("pretend"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err = besideInstallerRelative(installer, artifactNames("theia-server"))
	if err != nil {
		t.Fatal(err)
	}
	if found != plain {
		t.Errorf("found %q, want the plain name %q", found, plain)
	}
}
