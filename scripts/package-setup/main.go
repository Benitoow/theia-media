// package-setup appends a complete platform ZIP to the setup executable.
// The resulting single download remains executable and is also readable by
// archive/zip, so setup can install without fetching separate components.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	setup := flag.String("setup", "", "plain setup executable")
	archive := flag.String("archive", "", "complete platform ZIP")
	output := flag.String("out", "", "self-contained setup executable")
	flag.Parse()
	if *setup == "" || *archive == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: package-setup -setup <plain setup> -archive <complete ZIP> -out <standalone setup>")
		os.Exit(2)
	}
	if err := pack(*setup, *archive, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func pack(setup, archive, output string) error {
	if samePath(setup, output) || samePath(archive, output) {
		return fmt.Errorf("output must differ from both inputs")
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}
	defer z.Close()
	name := filepath.Base(setup)
	if !strings.HasPrefix(name, "theia-setup-") {
		return fmt.Errorf("setup has no platform asset name: %s", name)
	}
	plain := "theia-setup"
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		plain += ".exe"
	}
	server := "theia-server"
	if strings.HasSuffix(plain, ".exe") {
		server += ".exe"
	}
	for _, required := range []string{plain, server} {
		found := false
		for _, file := range z.File {
			if strings.TrimPrefix(file.Name, "./") == required {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("payload lacks %s", required)
		}
	}
	input, err := os.Open(setup)
	if err != nil {
		return err
	}
	defer input.Close()
	payload, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer payload.Close()
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".theia-setup-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := io.Copy(tmp, input); err != nil {
		return err
	}
	if _, err := io.Copy(tmp, payload); err != nil {
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), output); err != nil {
		return err
	}
	packed, err := zip.OpenReader(output)
	if err != nil {
		return fmt.Errorf("read packaged setup: %w", err)
	}
	defer packed.Close()
	if len(packed.File) != len(z.File) {
		return fmt.Errorf("packaged setup holds %d entries, expected %d", len(packed.File), len(z.File))
	}
	return nil
}

func samePath(a, b string) bool {
	x, e1 := filepath.Abs(a)
	y, e2 := filepath.Abs(b)
	return e1 == nil && e2 == nil && strings.EqualFold(x, y)
}
