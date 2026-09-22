package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Removing an installation.
//
// It takes away what the installer put on the machine - the entries in the Start
// Menu and on the Desktop, the autostart entry, the record in the applications
// list, and the programs - and it deliberately keeps the **data directory**. That
// is the library, the watch history and the configuration; a program that deletes
// somebody's watch history while "removing itself" is a program nobody forgives,
// and the folder is one line to delete by hand for anybody who wants it gone.
//
// Where the data is, and that it is still there, is part of what this prints: an
// uninstall that says nothing about it leaves somebody wondering whether their
// library went with it.

// The name this installation registers under, and the name Install and Uninstall
// actually use.
//
// The second is a variable for the same reason the shortcut targets are
// arguments: the registry has no APPDATA to redirect, and a test that wrote the
// real entry would register - or worse, unregister - the machine it is running
// on. A test points it at a name it owns and deletes the key afterwards.
const applicationKeyName = "Theia"

var registeredName = applicationKeyName

// Uninstall undoes an installation.
func Uninstall(plan Plan, targets ShortcutTargets, text Catalogue) (Result, error) {
	result := Result{
		Role:     plan.Role,
		DataDir:  plan.DataDir,
		Port:     plan.Port,
		Hostname: plan.Hostname,
		Library:  plan.LibraryPaths,
		Actions:  removeShortcuts(targets, text),
	}

	// The autostart entry first: it starts the server, and a server starting from
	// a file that is about to disappear is worse than one that does not start.
	if removed, err := RemoveAutostart(); err != nil {
		return result, err
	} else if removed.Kind != "" {
		result.Actions = append(result.Actions, Action{Kind: "removed-service", Detail: removed.Kind})
	}

	keyPath := applicationKeyPath(registeredName)
	if err := unregisterApplication(keyPath); err != nil {
		// Reported rather than fatal: the programs still have to go, and a
		// registry key nobody can write is not a reason to leave a product on
		// somebody's disk.
		result.ShortcutsError = err.Error()
	} else {
		result.Actions = append(result.Actions, Action{Kind: "unregistered-application", Path: keyPath})
	}

	// The two names on the machine go with the programs: a PATH entry pointing
	// at a folder that is about to be deleted is a command that exists and
	// fails, and an App Paths entry is the same promise in the Run dialog. Both
	// are Windows mechanisms; elsewhere the names are the two entries above.
	if isWindowsPath() {
		if removed, err := removeFromUserPath(plan.InstallDir); err != nil {
			result.ShortcutsError = joinReasons(result.ShortcutsError, err.Error())
		} else if removed {
			result.Actions = append(result.Actions, Action{Kind: "removed-from-path", Path: plan.InstallDir})
		}
		launcher := programExecutable(launcherBase)
		if appPathIsRegistered(launcher) {
			if err := unregisterAppPath(launcher); err != nil {
				result.ShortcutsError = joinReasons(result.ShortcutsError, err.Error())
			} else {
				result.Actions = append(result.Actions, Action{Kind: "unregistered-app-path", Path: launcher})
			}
		}
	}

	// And the entries under the home directory, which is where a macOS
	// installation keeps them: a symlink pointing at a folder that is about to be
	// deleted is the same broken promise.
	result.Actions = append(result.Actions, removeEntries(runtime.GOOS)...)

	result.Actions = append(result.Actions, removePrograms(plan, text)...)

	// And what was deliberately left alone, as an action like everything else:
	// "your library is still there, here" is the one thing somebody removing a
	// program wants to be sure of, and a result that carried it only in a field
	// would leave the terminal to remember to say it.
	result.Actions = append(result.Actions, Action{Kind: "kept-data", Path: plan.DataDir})
	return result, nil
}

// removeShortcuts deletes the entries this installation writes, and the folder
// that holds them once it is empty. Entries it does not recognise are left alone:
// the folder is the product's, but somebody else's shortcut in it is not.
func removeShortcuts(targets ShortcutTargets, text Catalogue) []Action {
	actions := make([]Action, 0, 4)
	folder := filepath.Join(targets.StartMenu, text["shortcutTheiaName"])

	if strings.TrimSpace(targets.StartMenu) != "" {
		for _, name := range shortcutNames(text) {
			path := filepath.Join(folder, name+".lnk")
			if !fileExists(path) {
				continue
			}
			if err := os.Remove(path); err == nil {
				actions = append(actions, Action{Kind: "removed-shortcut", Path: path})
			}
		}
		// Only succeeds when nothing else is in it, which is the point: a folder
		// holding somebody else's entry stays.
		os.Remove(folder)
	}

	if strings.TrimSpace(targets.Desktop) != "" {
		path := filepath.Join(targets.Desktop, text["shortcutTheiaName"]+".lnk")
		if fileExists(path) {
			if err := os.Remove(path); err == nil {
				actions = append(actions, Action{Kind: "removed-shortcut", Path: path})
			}
		}
	}
	return actions
}

// shortcutNames is every entry name this project writes, so a removal finds the
// ones an older or different role left behind rather than only the current
// role's.
func shortcutNames(text Catalogue) []string {
	return []string{
		text["shortcutTheiaName"],
		text["shortcutServerName"],
		text["shortcutPlayerName"],
	}
}

// removePrograms empties the installation directory.
//
// A tool run from its own installation cannot delete itself: Windows holds the
// executable open, so the file refuses to go while the process that is removing
// it is running. What cannot be deleted now is scheduled for the next start, and
// the result says so instead of claiming a clean removal that did not happen.
func removePrograms(plan Plan, text Catalogue) []Action {
	dir := plan.InstallDir
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return []Action{{Kind: "nothing-installed", Path: dir}}
	}

	actions := make([]Action, 0, 2)
	if err := os.RemoveAll(dir); err == nil {
		return append(actions, Action{Kind: "removed-program", Path: dir})
	}

	self, selfErr := os.Executable()
	if selfErr != nil || !within(self, dir) {
		// Something else is holding a file, and it is not this tool: say what is
		// left rather than pretending the folder went.
		return append(actions, Action{Kind: "programs-remaining", Path: dir})
	}

	_ = self
	if err := removeAfterExit(dir); err != nil {
		return append(actions, Action{Kind: "programs-remaining", Path: dir})
	}
	return append(actions, Action{Kind: "removed-program-later", Path: dir})
}

// within reports whether a file lives inside a directory.
func within(file, dir string) bool {
	relative, err := filepath.Rel(dir, file)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// installerExecutable is the name the installed copy of this tool has, used to
// avoid copying it over itself and to name it in the applications list.
func installerExecutable(goos string) string {
	if goos == "windows" {
		return "theia-setup.exe"
	}
	return "theia-setup"
}

// copySelf puts the maintenance tool beside the programs.
//
// The entry Windows keeps has to name a command that will still exist in a year,
// and the folder somebody downloaded into is not that place: it is emptied by
// housekeeping, moved, or deleted once the installation looks finished. The copy
// also makes `--uninstall` reachable from the applications list without keeping
// the original download.
func copySelf(target string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("setup: finding this program: %w", err)
	}
	if same, err := filepath.Abs(self); err == nil {
		if absolute, err := filepath.Abs(target); err == nil && strings.EqualFold(same, absolute) {
			return nil
		}
	}
	return copyInto(self, filepath.Dir(target), filepath.Base(target))
}
