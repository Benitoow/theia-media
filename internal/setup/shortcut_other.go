//go:build !windows

package setup

import "errors"

// Shortcuts elsewhere.
//
// A .lnk is a Windows shell object; Linux has .desktop files and macOS has .app
// bundles, and neither is the same file with a different extension. Writing one
// of those from here would be a second, unverified format pretending to be this
// feature, so the three symbols exist only to keep the package compiling for the
// five targets CI cross-compiles - with the same signatures, and an error that
// says what happened rather than a silent success.
var errShortcutsAreWindows = errors.New("setup: shortcuts are only created on Windows")

// Shortcut is declared here as well as in shortcut_windows.go because exactly one
// of the two files is compiled, and the type belongs to both APIs. The two
// declarations have to stay identical.
type Shortcut struct {
	Path        string // the .lnk file to write, absolute
	Target      string // the executable it starts
	Arguments   string
	WorkingDir  string
	Description string
	Icon        string // optional .ico or .exe; ignored when empty
}

// WriteShortcut creates the shortcut, replacing any file already there.
func WriteShortcut(Shortcut) error { return errShortcutsAreWindows }

// StartMenuDir is where a per-user Start Menu entry belongs.
func StartMenuDir() (string, error) { return "", errShortcutsAreWindows }

// DesktopDir is the user's Desktop.
func DesktopDir() (string, error) { return "", errShortcutsAreWindows }
