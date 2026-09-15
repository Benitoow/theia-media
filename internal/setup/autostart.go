package setup

import (
	"os"
	"path/filepath"
	"runtime"
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
