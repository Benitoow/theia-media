// Package layout is where the product's programs live inside an installation.
//
// It exists because that fact was written twice and only one copy knew about
// macOS. `internal/setup` has always known that a Mac player is the file inside
// `Theia.app`; `cmd/theia` looked for a loose `theia-player` beside itself, so on
// macOS the command started the server, found no player, and opened a browser
// instead of the film. Measured on the `macos-15-intel` runner on 28 September
// 2026: `PASS the theia command brought the installed server up by itself`,
// `FAIL the theia command did not open the player`.
//
// The answers are a function of the target platform rather than a build tag, so
// a test on any host can ask what a Mac installation is told (the lesson of
// decision 149). The package is deliberately small and dependency-free:
// `cmd/theia` links it, and the launcher ships as a few megabytes on purpose
// (decision 139).
package layout

import "path"

// PlayerExecutable is the file a shell runs for the player, relative to the
// installation directory.
//
// On Windows and Linux the player is a program beside its engine. On macOS it is
// an application bundle - which is what gives the window an identity and a Dock
// icon - and the file a shell can run is the one inside `Contents/MacOS`.
func PlayerExecutable(goos string) string {
	switch goos {
	case "darwin":
		return path.Join("Theia.app", "Contents", "MacOS", "theia-player")
	case "windows":
		return "theia-player.exe"
	default:
		return "theia-player"
	}
}

// ServerExecutable is the same for the server, which is a loose program on every
// platform that ships one - it has no window, so it has no bundle.
func ServerExecutable(goos string) string {
	if goos == "windows" {
		return "theia-server.exe"
	}
	return "theia-server"
}
