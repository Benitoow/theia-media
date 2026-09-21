//go:build windows

package setup

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// The two places Windows looks a command up by name, and why an installation
// writes both.
//
// **PATH is what a terminal reads.** Without it, `theia`, `theia-server` and
// `theia-player` are names that exist only inside their folder, and the
// maintainer's own words were that the programs could not be started from a
// terminal at all. The entry this file adds is one directory, removed again by
// --uninstall, and the value's own type is preserved: rewriting an expandable
// PATH as a plain string would stop every %USERPROFILE% inside it from expanding.
//
// **App Paths is what the Run dialog reads**, and what a dock or a launcher that
// resolves a name before it has been typed twice reads. It is how `Win+R theia`
// answers, which is the same question decision 123 answered for the Start Menu
// and the applications list.
//
// Both are HKEY_CURRENT_USER, because decision 120 fixes that this installer asks
// for no administrator rights.

// Where the two live, and how to read them.
//
// The key paths are variables for the same reason registeredName is: the
// registry has no APPDATA to redirect, so a test that wrote the real ones would
// change the machine it runs on - here, by putting a directory on somebody's
// PATH or teaching their Run dialog a program it does not have.
const (
	userPathKeyDefault = `Environment`
	userPathValue      = `Path`
	appPathsKeyDefault = `Software\Microsoft\Windows\CurrentVersion\App Paths`
)

var (
	userPathKey = userPathKeyDefault
	appPathsKey = appPathsKeyDefault
)

// userPath is what this user's PATH currently is, and the type it is stored as.
//
// A key or a value that is not there is an empty PATH and not a failure: a fresh
// Windows account has no Environment key until something writes one, and an
// installer is one of the things that may.
func userPath() (string, uint32, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, userPathKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", registry.EXPAND_SZ, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("setup: opening %s: %w", userPathKey, err)
	}
	defer key.Close()

	value, kind, err := key.GetStringValue(userPathValue)
	if errors.Is(err, registry.ErrNotExist) {
		return "", registry.EXPAND_SZ, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("setup: reading %s: %w", userPathValue, err)
	}
	return value, kind, nil
}

// addToUserPath puts dir on this user's PATH, and says whether it changed
// anything.
//
// An entry that is already there is left exactly as it is: running an installer
// twice must not put the same directory on somebody's PATH twice, and a PATH
// that has grown a duplicate of every program is a PATH somebody has to clean by
// hand.
func addToUserPath(dir string) (bool, error) {
	value, kind, err := userPath()
	if err != nil {
		return false, err
	}
	if _, found := withoutPathEntry(value, dir); found {
		return false, nil
	}
	if err := writeUserPath(appendPathEntry(value, dir), kind); err != nil {
		return false, err
	}
	return true, nil
}

// removeFromUserPath takes the directory back out, and says whether it was
// there. Every other entry is written back exactly as it was read, including the
// stray separators somebody's PATH has collected.
func removeFromUserPath(dir string) (bool, error) {
	value, kind, err := userPath()
	if err != nil {
		return false, err
	}
	kept, found := withoutPathEntry(value, dir)
	if !found {
		return false, nil
	}
	if err := writeUserPath(kept, kind); err != nil {
		return false, err
	}
	return true, nil
}

// writeUserPath stores the value, and tells the running programs that it changed.
func writeUserPath(value string, kind uint32) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, userPathKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("setup: opening %s: %w", userPathKey, err)
	}
	defer key.Close()

	if strings.TrimSpace(value) == "" {
		// An empty PATH is not a PATH. The value goes, which is the state this
		// key was in before the installation, and what Windows itself leaves
		// behind when the last entry is deleted.
		if err := key.DeleteValue(userPathValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("setup: removing %s: %w", userPathValue, err)
		}
	} else if err := setPathValue(key, value, kind); err != nil {
		return fmt.Errorf("setup: writing %s: %w", userPathValue, err)
	}

	// Without this the new PATH is in the registry and nowhere else: a terminal
	// opened a second later still cannot find the command, which reads as an
	// installation that did not work. Explorer is the process that matters here,
	// and it is the one listening for this message.
	broadcastEnvironmentChange()
	return nil
}

// setPathValue writes the PATH back with the type it was read as, because a
// %USERPROFILE% inside somebody's PATH only expands while the value is
// expandable. The registry package has one setter per type, and SZ is the answer
// for anything else this key could hold.
func setPathValue(key registry.Key, value string, kind uint32) error {
	if kind == registry.EXPAND_SZ {
		return key.SetExpandStringValue(userPathValue, value)
	}
	return key.SetStringValue(userPathValue, value)
}

// withoutPathEntry returns the value without this directory, and whether it was
// in it. Comparison is case-insensitive and ignores a trailing separator,
// because Windows writes both spellings and they are the same directory.
func withoutPathEntry(value, dir string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return value, false
	}
	entries := strings.Split(value, ";")
	kept := make([]string, 0, len(entries))
	found := false
	for _, entry := range entries {
		// An empty entry means the working directory, which is a real PATH
		// entry however unwise: it is not this function's business to drop it.
		if entry != "" && samePath(entry, dir) {
			found = true
			continue
		}
		kept = append(kept, entry)
	}
	if !found {
		return value, false
	}
	return strings.Join(kept, ";"), true
}

// appendPathEntry adds the directory as the last entry of the value.
func appendPathEntry(value, dir string) string {
	trimmed := strings.TrimRight(value, ";")
	if strings.TrimSpace(trimmed) == "" {
		return dir
	}
	return trimmed + ";" + dir
}

// samePath compares two PATH entries the way Windows does.
func samePath(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	trim := func(path string) string { return strings.TrimRight(path, `\/`) }
	return strings.EqualFold(left, right) || strings.EqualFold(trim(left), trim(right))
}

// registerAppPath teaches Windows that a word answers with a program.
//
// The default value is the command; the `Path` value beside it is the directory
// a launcher starts it in, which is what the shortcuts set too, so a relative
// path inside either program's configuration resolves the same way however it
// was started.
func registerAppPath(name, target string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, appPathsKey+`\`+name, registry.WRITE)
	if err != nil {
		return fmt.Errorf("setup: creating the App Paths entry for %s: %w", name, err)
	}
	defer key.Close()

	if err := key.SetStringValue("", target); err != nil {
		return fmt.Errorf("setup: writing the App Paths entry for %s: %w", name, err)
	}
	if err := key.SetStringValue("Path", filepath.Dir(target)); err != nil {
		return fmt.Errorf("setup: writing the App Paths directory for %s: %w", name, err)
	}
	return nil
}

// unregisterAppPath removes it again. A key that is not there is not an error:
// removing something twice is what a person does.
func unregisterAppPath(name string) error {
	err := registry.DeleteKey(registry.CURRENT_USER, appPathsKey+`\`+name)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("setup: removing the App Paths entry for %s: %w", name, err)
	}
	return nil
}

// appPathIsRegistered is what makes an uninstall able to say what it removed
// rather than what it tried to remove.
func appPathIsRegistered(name string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, appPathsKey+`\`+name, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	key.Close()
	return true
}

// broadcastEnvironmentChange tells every window that the environment changed.
//
// SendMessageTimeout rather than SendMessage, with ABORTIFHUNG: one window that
// has stopped pumping messages must not stop the installer, and the answers are
// not read - the point is only that every window that cares receives the
// message. The timeout is generous because a busy desktop is normal.
//
// It is not one of the functions golang.org/x/sys/windows wraps, so it is
// resolved here the way CoCreateInstance is in shortcut_windows.go.
var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
)

const (
	hwndBroadcast   = 0xFFFF
	wmSettingChange = 0x001A
	smtoAbortIfHung = 0x0002
)

func broadcastEnvironmentChange() {
	target := windows.StringToUTF16Ptr("Environment")
	var result uintptr
	procSendMessageTimeout.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(target)),
		smtoAbortIfHung,
		5000,
		uintptr(unsafe.Pointer(&result)),
	)
	runtime.KeepAlive(target)
}
