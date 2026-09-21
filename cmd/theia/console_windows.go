//go:build windows

package main

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// console is where this program says things, and it exists because the build has
// no console of its own (-H=windowsgui, see the package comment).
//
// Three cases, in the order they are tried. Started from a terminal - or with
// its output redirected into a file or a pipe, which is how the release pipeline
// reads it - the standard output it inherited is real, and it is used as it is.
// Started by a double-click there is no console at all, and AttachConsole
// borrows the parent's, which is how `theia -version` prints in the window
// somebody typed the command in. Started by Explorer, which has no console
// either, there is nowhere to write and this answers io.Discard: silence is what
// a double-click asked for, not a failure to report.
func console() io.Writer {
	if _, err := os.Stdout.Stat(); err == nil {
		return os.Stdout
	}
	if err := attachParentConsole(); err == nil {
		if output, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
			return output
		}
	}
	return io.Discard
}

// attachParentConsole is not one of the functions golang.org/x/sys/windows
// wraps, so it is resolved here the way internal/setup resolves
// CoCreateInstance. ATTACH_PARENT_PROCESS is the console of whatever started
// this program: the one a person is looking at when they typed the command.
var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
)

// attachParentProcess is (DWORD)-1: the parent's console rather than a process
// id, which is what makes this work without knowing who the parent is.
const attachParentProcess = 0xFFFFFFFF

func attachParentConsole() error {
	value, _, callErr := procAttachConsole.Call(uintptr(attachParentProcess))
	if value == 0 {
		return callErr
	}
	return nil
}
