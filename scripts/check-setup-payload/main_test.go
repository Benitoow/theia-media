package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestMacPayloadWithDotPrefixes(t *testing.T) {
	dir := t.TempDir()
	setup := filepath.Join(dir, "theia-setup-darwin-arm64")
	f, err := os.Create(setup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("mock executable"); err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, name := range []string{
		"START-HERE.txt", "theia-server", "theia-setup", "theia",
		"Theia.app/Contents/MacOS/theia-player",
		"Theia.app/Contents/Resources/LICENSE-libmpv.txt",
		"Theia.app/Contents/Resources/NOTICE.md",
		"Theia.app/Contents/Frameworks/libmpv.2.dylib",
	} {
		entry, err := z.Create("./" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("payload " + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	server := filepath.Join(dir, "theia-server-darwin-arm64")
	if err := os.WriteFile(server, []byte("payload theia-server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := check(setup, server, "darwin-arm64"); err != nil {
		t.Fatal(err)
	}
}
