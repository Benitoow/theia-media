//go:build !windows

package setup

// PATH and App Paths are Windows mechanisms, and elsewhere this installer writes
// no entries at all yet - decision 122 records why: a .desktop file or a shell
// profile written by a guess would be an unverified claim about somebody's menu.
// The programs are installed; these say so by doing nothing.

func addToUserPath(string) (bool, error)      { return false, nil }
func removeFromUserPath(string) (bool, error) { return false, nil }
func registerAppPath(string, string) error    { return nil }
func unregisterAppPath(string) error          { return nil }
func appPathIsRegistered(string) bool         { return false }
