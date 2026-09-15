//go:build windows

package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

// The entry that makes this an installed application rather than a folder.
//
// Windows asks every program the same question - "how do I remove you?" - and a
// program that cannot answer it is a folder somebody has to hunt for. It is what
// "Programs and Features", the Settings application page, and a launcher's
// registry source all read. Without it Theia was installed and invisible:
// present on disk, absent from the list of things on the machine.
//
// The key is under HKEY_CURRENT_USER, so no administrator rights are involved -
// the rule the autostart entry already follows (decision 120). A machine-wide
// entry would need elevation and would put one user's installation in every
// user's list.
const (
	// uninstallKeyRoot is where Windows looks for the list of installed
	// applications, per user.
	uninstallKeyRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall`

	// UninstallFlag is the command Windows runs to remove this installation.
	// Named here rather than typed into main, so the registered command and the
	// flag that answers it cannot drift apart.
	UninstallFlag = "--uninstall"
)

// Application is how this installation appears to Windows.
type Application struct {
	Name      string
	Version   string
	Publisher string
	// InstallLocation is the folder the programs live in, which Windows shows
	// and uses for "open file location".
	InstallLocation string
	// Icon is a file Windows reads an icon from, usually the program itself.
	Icon string
	// Uninstall is the command line Windows runs to remove the installation.
	Uninstall string
	// SizeKB is what the application list shows as the size on disk.
	SizeKB int64
}

// defaultApplication is what this installation registers itself as.
//
// The publisher is the project rather than a person: the repository is the
// identity a release has, and inventing a company name would be a claim nobody
// can check. The icon is the player when it is installed, because that is the
// program with a window, and the server otherwise.
func defaultApplication(plan Plan, version, uninstaller string) Application {
	return Application{
		Name:            applicationKeyName,
		Version:         version,
		Publisher:       "Theia",
		InstallLocation: plan.InstallDir,
		Icon:            firstInstalled(plan.InstallDir, "theia-player", "theia-server"),
		Uninstall:       uninstallCommand(uninstaller),
		SizeKB:          directorySizeKB(plan.InstallDir),
	}
}

// uninstallCommand quotes the path the way Windows reads it: plain double quotes
// around it, not Go's %q, which would escape every backslash and leave the list
// with a command that cannot be run. Written once because Programs and Features
// runs this string through the shell, and most installation folders have a space
// in them.
func uninstallCommand(uninstaller string) string {
	return `"` + uninstaller + `" ` + UninstallFlag
}

// registerApplication writes the entry under a given key path.
//
// The path is an argument rather than a constant so a test can write under a key
// it owns and delete it again: the registry has no APPDATA to redirect, and a
// test that wrote to the real uninstall list would change the machine it runs on.
func registerApplication(keyPath string, app Application) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("setup: creating %s: %w", keyPath, err)
	}
	defer key.Close()

	values := map[string]string{
		"DisplayName":     app.Name,
		"DisplayVersion":  app.Version,
		"Publisher":       app.Publisher,
		"InstallLocation": app.InstallLocation,
		"InstallDate":     time.Now().Format("20060102"),
	}
	if app.Icon != "" {
		values["DisplayIcon"] = app.Icon
	}
	if app.Uninstall != "" {
		values["UninstallString"] = app.Uninstall
		// The same command twice: this uninstall asks no question, so there is
		// no separate quiet form to offer.
		values["QuietUninstallString"] = app.Uninstall
	}
	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("setup: writing %s: %w", name, err)
		}
	}
	// No "modify" and no "repair": there is no such mode, and a button that does
	// nothing is worse than a button that is not there.
	for _, name := range []string{"NoModify", "NoRepair"} {
		if err := key.SetDWordValue(name, 1); err != nil {
			return fmt.Errorf("setup: writing %s: %w", name, err)
		}
	}
	if app.SizeKB > 0 {
		if err := key.SetDWordValue("EstimatedSize", uint32(app.SizeKB)); err != nil {
			return fmt.Errorf("setup: writing EstimatedSize: %w", err)
		}
	}
	return nil
}

// unregisterApplication removes the entry, and the key with it. A key that is not
// there is not an error: removing something twice is what a person does.
func unregisterApplication(keyPath string) error {
	if err := registry.DeleteKey(registry.CURRENT_USER, keyPath); err != nil &&
		!errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("setup: removing %s: %w", keyPath, err)
	}
	return nil
}

// applicationIsRegistered reports whether this installation is in the list, which
// is what `--check` answers with.
func applicationIsRegistered(keyPath string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue("DisplayName")
	return err == nil
}

// applicationKeyPath is where this installation's entry lives.
func applicationKeyPath(name string) string {
	return uninstallKeyRoot + `\` + name
}

// directorySizeKB is what Windows shows as the size on disk. A folder that cannot
// be walked reports zero, and the list simply omits the size.
func directorySizeKB(dir string) int64 {
	var total int64
	filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total / 1024
}

// removeAfterExit starts a command that removes a folder once this process has
// ended, and returns whether it was started.
//
// Windows will not let a running executable be deleted, and it will not let an
// **unelevated** process schedule that deletion for the next start either:
// MOVEFILE_DELAY_UNTIL_REBOOT writes under HKEY_LOCAL_MACHINE, which this
// installer never has the right to do - decision 120 fixes that it asks for
// none. The first version of this used it, and the real uninstall answered "some
// files could not be removed" about a folder holding nothing but the tool that
// was running from it.
//
// So the process that does the work has to outlive the one asking for it: a small
// command file waits a second, by which time this program has exited, and then
// removes the folder and itself. Three details are load-bearing, and every one of
// them came from the test that drives this:
//
//   - The command lives in a **file**, not in an argument. Go quotes arguments the
//     way a C runtime expects - an inner quote escaped with a backslash - which
//     cmd.exe passes through literally, so a path with a space in it arrives
//     mangled and rmdir then deletes nothing: silently, with exit status 0. Both
//     attempts at an argument, and one at standard input, failed exactly that way
//     before this shape was measured against a real folder.
//   - The file name is random (`os.CreateTemp`), so nothing can wait at a
//     predictable path in the temporary folder for a tool that runs commands.
//   - The wait is a ping rather than `timeout`, because `timeout` needs a console
//     and this runs with no window.
func removeAfterExit(dir string) error {
	script, err := os.CreateTemp("", "theia-uninstall-*.cmd")
	if err != nil {
		return fmt.Errorf("setup: creating the removal script: %w", err)
	}
	path := script.Name()
	body := "@echo off\r\n" +
		"rem Written by theia-setup: removes an installation once the tool that asked has exited.\r\n" +
		"ping -n 2 127.0.0.1 >nul\r\n" +
		"rmdir /s /q \"" + dir + "\"\r\n" +
		"del \"%~f0\"\r\n"
	if _, err := script.WriteString(body); err != nil {
		script.Close()
		os.Remove(path)
		return fmt.Errorf("setup: writing the removal script: %w", err)
	}
	if err := script.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("setup: closing the removal script: %w", err)
	}

	command := exec.Command("cmd.exe", "/c", path)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := command.Start(); err != nil {
		os.Remove(path)
		return fmt.Errorf("setup: starting the removal of %s: %w", dir, err)
	}
	// The child is not waited for: waiting would be waiting for our own deletion,
	// and it has to outlive us.
	return command.Process.Release()
}

// createNoWindow is CREATE_NO_WINDOW: the helper is a detail of an uninstall, and
// a console flashing up while a program removes itself is alarming.
const createNoWindow = 0x08000000
