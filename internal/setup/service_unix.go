//go:build !windows

package setup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Autostart on macOS and Linux, as *user* agents, for the same reason Windows
// gets a per-user entry when it cannot have a task: nothing here needs root, and
// installing a machine-wide service is a decision about somebody's computer that
// an installer has no business making on its own.
//
// What is verified here is what can be verified on the machine this project is
// developed on: the path each entry is written to and the text it contains are
// built in autostart.go, where the Windows test suite reaches them
// (TestTheLaunchdAgentSaysWhatTheInstallationDoes). What is **not** verified is
// the platform's half - that launchd and systemd accept these files, and that
// `launchctl load` and `systemctl --user enable` succeed - which only a Mac and a
// Linux machine can confirm.
const (
	autostartSystemdUnit = "systemd-user-unit"
	autostartLaunchAgent = "launchd-agent"
)

func installAutostart(plan Plan, server string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return autostartLaunchAgent, writeLaunchAgent(plan, server)
	default:
		return autostartSystemdUnit, writeSystemdUnit(plan, server)
	}
}

func removeAutostart() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		path, err := launchAgentPath()
		if err != nil {
			return "", err
		}
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", err
		}
		// Best effort: unloading needs a session bus, which a machine being
		// installed over SSH does not have. The file is what makes it start.
		_ = exec.Command("launchctl", "unload", path).Run()
		return autostartLaunchAgent, nil
	default:
		path, err := systemdUnitPath()
		if err != nil {
			return "", err
		}
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", err
		}
		if _, err := exec.LookPath("systemctl"); err == nil {
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		}
		return autostartSystemdUnit, nil
	}
}

func autostartStatus() Autostart {
	switch runtime.GOOS {
	case "darwin":
		path, err := launchAgentPath()
		if err != nil {
			return Autostart{}
		}
		if _, err := os.Stat(path); err == nil {
			return Autostart{Kind: autostartLaunchAgent, Path: path, Installed: true}
		}
	default:
		path, err := systemdUnitPath()
		if err != nil {
			return Autostart{}
		}
		if _, err := os.Stat(path); err == nil {
			return Autostart{Kind: autostartSystemdUnit, Path: path, Installed: true}
		}
	}
	return Autostart{}
}

func writeSystemdUnit(plan Plan, server string) error {
	path, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := writeAutostartFile(path, systemdUnit(launchCommand(plan, server))); err != nil {
		return err
	}
	// Enabled when a session bus is there to talk to. Over SSH, or before the
	// user has ever logged in, systemctl cannot reach one - and saying so is
	// better than failing the installation over it.
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", systemdUnitName).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user enable: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// writeLaunchAgent writes the agent and hands it to launchd.
//
// The text and the path are built in autostart.go, where a test on any platform
// can reach them; what is left here is the part that needs a Mac: the file itself
// and `launchctl`, which loads it now so that installing the autostart entry does
// not require a reboot to mean anything.
func writeLaunchAgent(plan Plan, server string) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if err := writeAutostartFile(path, launchAgentPlist(launchCommand(plan, server))); err != nil {
		return err
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		return nil
	}
	// Unload first, best effort: a machine that already had this agent loaded
	// would otherwise keep running the copy the file used to describe.
	_ = exec.Command("launchctl", "unload", path).Run()
	if out, err := exec.Command("launchctl", "load", path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func systemdUnitPath() (string, error) {
	home, err := homeDirectory()
	if err != nil {
		return "", err
	}
	return systemdUnitPathIn(home), nil
}

func launchAgentPath() (string, error) {
	home, err := homeDirectory()
	if err != nil {
		return "", err
	}
	return launchAgentPathIn(home), nil
}

func serverExecutable() (string, error) {
	return findArtifact("theia-server")
}
