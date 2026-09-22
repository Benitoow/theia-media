package main

import (
	"os/exec"
	"runtime"
)

// openInBrowser hands a URL to whatever this machine uses for one.
//
// It exists for the machine that has a server and no player: a home server in a
// cupboard, or a Mac whose player has not been installed yet. Bare `theia` on
// that machine used to start the server and then say nothing at all, which reads
// as a command that did nothing - and the web interface, which the server
// already serves for administration and fallback playback, is the way in.
//
// It starts rather than waits: a browser outlives the command that opened it,
// and the handle is released so it is nobody else's child. The error is returned
// rather than fatal, because nothing about starting a server should depend on a
// browser existing - on a machine with no graphical session this refuses, and
// the address printed beside it is the whole answer.
func openInBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		// The shell's own handler, without a shell: rundll32 is what Explorer
		// runs for a URL, and `start` is a cmd.exe builtin that would need one.
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}
