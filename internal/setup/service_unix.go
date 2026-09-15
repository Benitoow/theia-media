//go:build !windows

package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Autostart on macOS and Linux, as *user* agents, for the same reason Windows
// gets a per-user entry when it cannot have a task: nothing here needs root, and
// installing a machine-wide service is a decision about somebody's computer that
// an installer has no business making on its own.
//
// Neither path is verified. Windows is the only platform V3.3 can be validated
// on (spec-fondatrice §14.2), so this file is written and reported as unverified
// rather than presented as working.
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	unit := fmt.Sprintf(`[Unit]
Description=Theia media server
Documentation=https://github.com/Benitoow/theia-media

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, strings.Join(quoteAll(launchCommand(plan, server)), " "))
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
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
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "theia.service").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user enable: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writeLaunchAgent(plan Plan, server string) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	args := launchCommand(plan, server)
	var list strings.Builder
	list.WriteString("\t\t<array>\n")
	list.WriteString("\t\t\t<string>" + args[0] + "</string>\n")
	for _, arg := range args[1:] {
		list.WriteString("\t\t\t<string>" + arg + "</string>\n")
	}
	list.WriteString("\t\t</array>\n")

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>media.theia.server</string>
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
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return err
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		return nil
	}
	_ = exec.Command("launchctl", "unload", path).Run()
	if out, err := exec.Command("launchctl", "load", path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "theia.service"), nil
}

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "media.theia.server.plist"), nil
}

func serverExecutable() (string, error) {
	if beside, err := besideInstaller("theia-server"); err == nil {
		return beside, nil
	}
	if onPath, err := exec.LookPath("theia-server"); err == nil {
		return onPath, nil
	}
	return "", fmt.Errorf("theia-server was not found beside the installer or on PATH")
}

func quoteAll(args []string) []string { return args }
