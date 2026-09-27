package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestPackAcceptsMacZipNamesAndProducesReadableExecutable(t *testing.T) {
	dir := t.TempDir()
	setup := filepath.Join(dir, "theia-setup-darwin-arm64")
	if err := os.WriteFile(setup, []byte("mock executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "payload.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, name := range []string{"./theia-setup", "./theia-server"} {
		entry, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "public", "theia-setup-darwin-arm64")
	if err := pack(setup, archive, output); err != nil {
		t.Fatal(err)
	}
	got, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	if len(got.File) != 2 {
		t.Errorf("packaged %d entries, want 2", len(got.File))
	}
}
