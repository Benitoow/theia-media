//go:build !windows

package main

import (
	"io"
	"os"
)

// console is where this program says things. Away from Windows it is an ordinary
// process with a terminal behind it, so there is nothing to arrange.
func console() io.Writer { return os.Stdout }
