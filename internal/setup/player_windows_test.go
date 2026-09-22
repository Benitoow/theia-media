//go:build windows

package setup

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The probe that keeps an update from fighting a running player, measured
// against a handle that really does hold the file.
//
// It is worth a test rather than an argument because the whole reason the probe
// exists is that Windows refuses the write, and only Windows can be asked. The
// handle below allows no sharing at all, which is what a running program's own
// image does to everything except deletion.
func TestBundleBlockerSeesAFileAnotherHandleHolds(t *testing.T) {
	dir := t.TempDir()
	names := bundleFiles("windows")
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("installed "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(dir, names[0])
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(pointer, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatalf("holding %s: %v", names[0], err)
	}
	defer syscall.CloseHandle(handle)

	name, held := bundleBlocker(dir, names)
	if name != names[0] || !held {
		t.Fatalf("bundleBlocker = %q/%v, want %q/true", name, held, names[0])
	}

	// And with nothing holding it, the same bundle is free to be replaced: a
	// probe that always answered "in use" would be a probe nobody could act on.
	syscall.CloseHandle(handle)
	if name, held := bundleBlocker(dir, names); name != "" || held {
		t.Fatalf("bundleBlocker without a holder = %q/%v, want empty/false", name, held)
	}
}
