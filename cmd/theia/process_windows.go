//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// exeSuffix is what an executable is called on this platform.
const exeSuffix = ".exe"

// createNoWindow is CREATE_NO_WINDOW, the flag that keeps a console program from
// opening a console. The server is one, and it is started from a program that
// has no console to hand it: without this flag a black window would appear
// beside the film - which is the fault this command's windowed build exists to
// avoid, one process further down.
const createNoWindow = 0x08000000

// spawnDetached starts a program and lets it outlive this one.
//
// Nothing is waited for: `theia` is a front door, and the processes behind it
// run for as long as the machine wants them to. The working directory is the
// installation, which is what the shortcuts set too, so a relative path in
// either program's own configuration resolves the same way however it was
// started.
func spawnDetached(path, dir string) error {
	command := exec.Command(path)
	command.Dir = dir
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	return command.Start()
}
