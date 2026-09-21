//go:build !windows

package main

import "os/exec"

// exeSuffix is empty where executables carry no extension.
const exeSuffix = ""

// spawnDetached starts a program and lets it outlive this one. Nothing is waited
// for: `theia` is a front door, and the processes behind it run for as long as
// the machine wants them to.
func spawnDetached(path, dir string) error {
	command := exec.Command(path)
	command.Dir = dir
	return command.Start()
}
