//go:build windows

package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Windows autostart, and the measurement that shaped it.
//
// The server binary is an argument rather than something looked up here: which
// binary to start is a decision the caller makes once, and a mechanism that
// guessed would start whatever happened to be beside it.
//
// The founding spec's §11.7 amendment names "a Windows scheduled task". Measured
// on the maintainer's machine from a non-elevated shell: `schtasks /create`
// answers "Accès refusé" - a task cannot be created without administrator
// rights. The same amendment says "aucune élévation imposée", and those two
// cannot both hold silently.
//
// So the tool installs what the shell it runs in can actually install:
//
//   - already elevated: a scheduled task, which is what the spec asks for;
//   - not elevated: a Startup entry, which needs no rights at all and runs at
//     logon like the task would.
//
// It never asks for elevation and never substitutes one mechanism for the other
// in silence: the result says which was used, and the status says why.
const (
	autostartScheduledTask = "scheduled-task"
	autostartStartupEntry  = "startup-entry"

	taskName = "Theia"
)

func installAutostart(plan Plan, server string) (string, error) {
	if isElevated() {
		if err := createScheduledTask(plan, server); err != nil {
			return "", err
		}
		return autostartScheduledTask, nil
	}
	if err := writeStartupEntry(plan, server); err != nil {
		return "", err
	}
	return autostartStartupEntry, nil
}

func removeAutostart() (string, error) {
	removed := ""
	// Both are attempted: an installation may have been made once from an
	// elevated shell and once from a normal one, and leaving either behind is a
	// server that starts twice.
	if path, err := startupEntryPath(); err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			if err := os.Remove(path); err != nil {
				return "", err
			}
			removed = autostartStartupEntry
		}
	}
	if exists, err := scheduledTaskExists(); err == nil && exists {
		if out, err := exec.Command("schtasks", "/delete", "/tn", taskName, "/f").CombinedOutput(); err != nil {
			return removed, fmt.Errorf("schtasks /delete: %w: %s", err, strings.TrimSpace(string(out)))
		}
		removed = autostartScheduledTask
	}
	return removed, nil
}

func autostartStatus() Autostart {
	if exists, err := scheduledTaskExists(); err == nil && exists {
		return Autostart{Kind: autostartScheduledTask, Installed: true}
	}
	path, err := startupEntryPath()
	if err != nil {
		return Autostart{}
	}
	if _, err := os.Stat(path); err == nil {
		status := Autostart{Kind: autostartStartupEntry, Path: path, Installed: true}
		if !isElevated() {
			status.Note = "a scheduled task needs administrator rights; this startup entry does not"
		}
		return status
	}
	return Autostart{}
}

// createScheduledTask registers the server to start at logon.
//
// `onlogon` rather than `onstart` on purpose: a task that runs before anybody
// logs in has to run as a system account, which is an administrator decision
// this tool exists to avoid making on somebody's behalf.
func createScheduledTask(plan Plan, server string) error {
	command := strings.Join(quoteAll(launchCommand(plan, server)), " ")
	out, err := exec.Command("schtasks",
		"/create", "/tn", taskName, "/tr", command, "/sc", "onlogon", "/f",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks /create: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func scheduledTaskExists() (bool, error) {
	err := exec.Command("schtasks", "/query", "/tn", taskName).Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// schtasks answers 1 with "the specified file was not found" when the
		// task does not exist. That is the answer, not a failure.
		return false, nil
	}
	return false, err
}

// writeStartupEntry drops a small launcher into the per-user Startup folder.
//
// It starts the server minimised: the server is a foreground program and prints
// where it is listening, so a window appears at every logon. Minimised it is out
// of the way and still there to be read when something goes wrong, which a
// hidden one would not be.
func writeStartupEntry(plan Plan, server string) error {
	path, err := startupEntryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	command := strings.Join(quoteAll(launchCommand(plan, server)), " ")
	content := "@echo off\r\n" +
		"rem Written by theia-setup. The server starts minimised; its window is the log.\r\n" +
		"start \"Theia\" /min " + command + "\r\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

func startupEntryPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA is not set, so the Startup folder cannot be found")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Theia.cmd"), nil
}

// serverExecutable is the binary the entry should start: beside the installer
// when it is there, otherwise the one on PATH.
func serverExecutable() (string, error) {
	return findArtifact("theia-server")
}

// isElevated reports whether this process can do administrator things. The
// installer uses it to choose a mechanism, never to demand anything.
func isElevated() bool {
	// The real token check, not a `net session` probe: that one prints a
	// localised refusal, so a French Windows answers differently from an English
	// one - which is exactly the sort of thing this project has been bitten by.
	return windows.GetCurrentProcessToken().IsElevated()
}

func quoteAll(args []string) []string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, `"`+arg+`"`)
	}
	return quoted
}
