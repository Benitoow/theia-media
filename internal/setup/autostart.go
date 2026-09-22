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
//
// The rule itself lives in diskNames, which takes the platform as an argument;
// this is the same rule asked about the machine the code is running on.
func artifactNames(base string) []string {
	return diskNames(base, runtime.GOOS, runtime.GOARCH)
}

// A startup entry on macOS and Linux: where it goes, and what it says.
//
// Both are text files written under the user's home directory, and the only part
// of installing one that is genuinely specific to its platform is the command
// that loads it - `launchctl load`, `systemctl --user enable`. So the two
// questions worth pinning, *where* the file goes and *what is in it*, are
// answered here rather than in service_unix.go, which is compiled on Unix only:
// this suite runs on Windows, and a claim about a launchd agent that no test on
// this machine can reach is a claim nobody has checked.
//
// Everything here is unverified on a real Mac. What it pins is that the file says
// what the code says it says; the Mac has to confirm that launchd agrees.
const (
	// launchAgentLabel names the job: reverse-DNS, the way launchd names them.
	launchAgentLabel = "media.theia.server"

	// systemdUnitName is the unit's file name, which is also the name
	// `systemctl --user enable` is given.
	systemdUnitName = "theia.service"
)

// launchAgentPathIn is where this user's launchd agent belongs: LaunchAgents,
// which launchd reads when the user logs in - per-user, never /Library/LaunchDaemons,
// which would need administrator rights and would start before anybody is there.
func launchAgentPathIn(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

// systemdUnitPathIn is where this user's systemd unit belongs.
func systemdUnitPathIn(home string) string {
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName)
}

// launchAgentPlist is the agent, as text.
//
// It runs the same argument vector the Windows startup entry and the systemd unit
// run, because all three have to start exactly the same thing. RunAtLoad starts
// it at login, and KeepAlive restarts it only when it failed - a server that
// exited cleanly is a server somebody stopped.
func launchAgentPlist(args []string) string {
	var list strings.Builder
	list.WriteString("\t\t<array>\n")
	list.WriteString("\t\t\t<string>" + args[0] + "</string>\n")
	for _, arg := range args[1:] {
		list.WriteString("\t\t\t<string>" + arg + "</string>\n")
	}
	list.WriteString("\t\t</array>\n")

	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchAgentLabel + `</string>
	<key>ProgramArguments</key>
` + list.String() + `	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
</dict>
</plist>
`
}

// systemdUnit is the unit, as text. Restart=on-failure and RestartSec are the
// same decision the launchd agent makes with KeepAlive.
func systemdUnit(args []string) string {
	return fmt.Sprintf(`[Unit]
Description=Theia media server
Documentation=https://github.com/Benitoow/theia-media

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, strings.Join(args, " "))
}

// writeAutostartFile writes one startup entry, creating the folder it lives in.
//
// It is here rather than beside the platforms that call it because writing a text
// file is not a platform-specific act, and a test that runs on Windows should be
// able to prove that the plist lands where launchd looks for it.
func writeAutostartFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
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
