//go:build !windows

package setup

import "fmt"

// The Windows application registration, absent elsewhere.
//
// V3.3 is verified on Windows only; a program list entry on a platform nobody has
// run would be a claim rather than a pin, which is the rule the whole project is
// held to. The programs are still installed, and the data directory is still
// theirs to move.

// UninstallFlag is the flag that removes an installation. It exists on every
// platform because the command line does; only the registration is Windows-only.
const UninstallFlag = "--uninstall"

// Application is how an installation appears to the operating system.
type Application struct {
	Name            string
	Version         string
	Publisher       string
	InstallLocation string
	Icon            string
	Uninstall       string
	SizeKB          int64
}

func defaultApplication(plan Plan, version, uninstaller string) Application {
	return Application{
		Name:            applicationKeyName,
		Version:         version,
		Publisher:       "Theia",
		InstallLocation: plan.InstallDir,
		Uninstall:       uninstaller + " " + UninstallFlag,
	}
}

// scheduleDeletionAtReboot has no equivalent here, and does not need one: on a
// Unix system a running file can be unlinked, so removing an installation never
// has to wait for a restart. Returning an error rather than nil keeps the caller
// honest - it reports what is left instead of claiming a clean removal.
func scheduleDeletionAtReboot(path string) error {
	return fmt.Errorf("setup: %s cannot be scheduled for deletion; there is no such mechanism here", path)
}

func registerApplication(string, Application) error { return nil }
func unregisterApplication(string) error            { return nil }
func applicationIsRegistered(string) bool           { return false }
func applicationKeyPath(name string) string         { return name }
func directorySizeKB(string) int64                  { return 0 }
