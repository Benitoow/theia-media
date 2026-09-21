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

	// PATH and App Paths are redirected in the same place, for the same reason:
	// a test that added a directory to the real PATH, or taught the real Run
	// dialog a program, would change the machine it runs on - which is the fault
	// this function already exists to prevent.
	userPathKey = `Software\Theia\tests\Environment`
	appPathsKey = `Software\Theia\tests\App Paths`

	code := m.Run()

	// Remove whatever the run left. The keys are walked rather than deleted,
	// because DeleteKey refuses one that still has subkeys: the Environment key
	// and the App Paths entries live under this root now, and the plain call
	// this used to make would leave both behind on whoever ran the suite.
	deleteKeyTree(keyPath)
	deleteKeyTree(`Software\Theia\tests`)
	deleteKeyTree(`Software\Theia`)

	// And report a leak: a test that wrote to the real entry would otherwise be
	// invisible until somebody looked at their own machine.
	if previous != testName && applicationIsRegistered(applicationKeyPath(previous)) {
		fmt.Fprintf(os.Stderr, "setup tests: the real applications-list entry %q was written during the run\n", previous)
	}
	os.Exit(code)
}

// deleteKeyTree removes a key and everything under it.
func deleteKeyTree(path string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return
	}
	names, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return
	}
	for _, name := range names {
		deleteKeyTree(path + `\` + name)
	}
	registry.DeleteKey(registry.CURRENT_USER, path)
}
