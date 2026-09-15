package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Autostart is what this machine does about starting the server by itself.
//
// The kinds are named rather than described: "startup-entry", "scheduled-task",
// "systemd-user-unit", "launchd-agent", or "" when there is none. The interface
// turns that into a sentence, because the sentence differs between the
// mechanisms and only the interface knows which language it is in.
type Autostart struct {
	Kind      string `json:"kind,omitempty"`
	Path      string `json:"path,omitempty"`
	Installed bool   `json:"installed"`
	// Note carries a platform fact the user has to know and the tool cannot fix:
	// on Windows, that a scheduled task needs administrator rights and a startup
	// entry was installed instead. Empty when there is nothing to add.
	Note string `json:"note,omitempty"`
}

// RemoveAutostart deletes whatever this installation put in place to start
// itself, whichever mechanism that was. Both Windows mechanisms are attempted,
// because an installation can have been made twice from shells with different
// rights, and leaving one behind is a server that starts twice.
func RemoveAutostart() (Autostart, error) {
	kind, err := removeAutostart()
	if err != nil {
		return Autostart{}, err
	}
	if kind == "" {
		return Autostart{}, nil
	}
	return Autostart{Kind: kind, Installed: false}, nil
}

// Command line helpers shared by the platform files.

// launchCommand is the argument vector that starts this installation's server.
// Kept in one place because a startup entry, a systemd unit and a launchd agent
// must all start exactly the same thing - and because the data directory has to
// be quoted for paths with spaces, which is most of them on Windows.
func launchCommand(plan Plan, server string) []string {
	return []string{server, "--data-dir", plan.DataDir}
}

// artifactNames lists what a binary may be called on disk, most specific first.
//
// Two names exist and both are real: `theia-server.exe` is what `build.ps1` and
// a working tree produce, and `theia-server-windows-amd64.exe` is what the
// release publishes. **A user only ever has the second one**, and the first
// version of this looked for the first only - so an installation with every
// published file sitting in one folder reported the server as missing and could
// not install its autostart entry. Found by downloading the release assets into
// an empty directory and running the installer the way a person would.
func artifactNames(base string) []string {
	extension := ""
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}
	return []string{
		base + extension,
		fmt.Sprintf("%s-%s-%s%s", base, runtime.GOOS, runtime.GOARCH, extension),
	}
}

// findArtifact returns the first of those names that is installed, beside the
// installer, or failing that on PATH.
//
// The installation directory comes first because it is what an installation
// means: the autostart entry and the shortcuts must start the copy this machine
// installed, not a copy that happens to sit next to the tool somebody ran - and
// somebody who runs the installer out of their Downloads folder has both.
func findArtifact(base string) (string, error) {
	names := artifactNames(base)
	if installDir, err := DefaultInstallDir(); err == nil {
		if path, err := inDirRelative(installDir, names); err == nil {
			return path, nil
		}
	}
	for _, name := range names {
		if path, err := besideInstaller(name); err == nil {
			return path, nil
		}
	}
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s was not found in the installation, beside the installer or on PATH (looked for %s)",
		names[0], strings.Join(names, ", "))
}

// besideInstaller finds a file sitting next to the running executable, which is
// where an unpacked release keeps its siblings.
func besideInstaller(name string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(self), name)
	if !fileExists(path) {
		return "", os.ErrNotExist
	}
	return path, nil
}

func isWindowsPath() bool { return runtime.GOOS == "windows" }
