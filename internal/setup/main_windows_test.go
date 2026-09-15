//go:build windows

package setup

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// TestMain keeps every test away from the machine's real applications-list entry.
//
// The registry has no APPDATA to redirect: a test that called Uninstall without
// pointing the name somewhere else would remove the entry of the machine it is
// running on, which is exactly what happened once - the installer's own tests
// unregistered the maintainer's installation, and `theia-setup --check` then
// reported "Application: no" about an installation that was still there. The
// redirection lives here rather than in each helper, so a test written later
// cannot forget it.
func TestMain(m *testing.M) {
	const testName = "Theia-tests"
	previous := registeredName
	registeredName = testName
	keyPath := applicationKeyPath(testName)

	code := m.Run()

	// Remove whatever the run left, and the parents if they are empty.
	registry.DeleteKey(registry.CURRENT_USER, keyPath)
	registry.DeleteKey(registry.CURRENT_USER, `Software\Theia\tests`)
	registry.DeleteKey(registry.CURRENT_USER, `Software\Theia`)

	// And report a leak: a test that wrote to the real entry would otherwise be
	// invisible until somebody looked at their own machine.
	if previous != testName && applicationIsRegistered(applicationKeyPath(previous)) {
		fmt.Fprintf(os.Stderr, "setup tests: the real applications-list entry %q was written during the run\n", previous)
	}
	os.Exit(code)
}
