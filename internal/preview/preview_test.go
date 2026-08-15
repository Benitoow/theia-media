package preview

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// absentFFmpeg is the ordinary state of a fresh installation: a library of
// browser-friendly files never downloads ffmpeg, so nothing may assume one.
type absentFFmpeg struct{ asked bool }

func (f *absentFFmpeg) Available() bool { return false }
func (f *absentFFmpeg) Path(context.Context) (string, error) {
	f.asked = true
	return "", errors.New("Path must not be called when ffmpeg is absent")
}

func newTestManager(t *testing.T, binary Binary) *Manager {
	t.Helper()
	m, err := New(t.TempDir(), binary, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestAskingForAPreviewNeverDownloadsFFmpeg(t *testing.T) {
	// The promise M1 made for /info and decision 58 restated for the encoder
	// probe: finding out what this machine can do must not be the thing that
	// fetches eighty megabytes.
	binary := &absentFFmpeg{}
	m := newTestManager(t, binary)

	_, err := m.Lookup(t.Context(), Key("/films/heat.mkv", 100, 7), "/films/heat.mkv", 9000)
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v, want ErrUnavailable", err)
	}
	if binary.asked {
		t.Error("the preview manager reached for the ffmpeg binary when none was on disk")
	}
}

func TestAShortFileIsNotWorthAStrip(t *testing.T) {
	m := newTestManager(t, &absentFFmpeg{})
	key := Key("/films/short.mkv", 10, 1)

	if _, err := m.Lookup(t.Context(), key, "/films/short.mkv", 30); !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v for a thirty-second file, want ErrUnavailable", err)
	}
}

func TestTheKeyFollowsTheFileRatherThanItsName(t *testing.T) {
	base := Key("/films/heat.mkv", 1000, 500)

	if same := Key("/films/heat.mkv", 1000, 500); same != base {
		t.Error("the same file produced two different keys")
	}
	// Replacing a film with a different encode under the same name must not
	// keep showing frames from the file that is gone.
	if Key("/films/heat.mkv", 2000, 500) == base {
		t.Error("a file of a different size kept the same key")
	}
	if Key("/films/heat.mkv", 1000, 900) == base {
		t.Error("a file modified since kept the same key")
	}
	if Key("/other/heat.mkv", 1000, 500) == base {
		t.Error("a file in another folder kept the same key")
	}
	if !keyPattern.MatchString(base) {
		t.Errorf("key %q is not in the form the sheet route accepts", base)
	}
}

func TestASheetIsOnlyServedForAKeyThatCouldBeOne(t *testing.T) {
	m := newTestManager(t, &absentFFmpeg{})

	// The key reaches the filesystem, so this is the one place in a
	// best-effort package that may not be relaxed.
	for _, bad := range []string{
		"../config",
		"..\\config",
		"/etc/passwd",
		"NOTHEX00000000000000000000000000",
		"",
		"abc",
	} {
		if _, err := m.SheetPath(bad); !errors.Is(err, ErrUnavailable) {
			t.Errorf("SheetPath(%q) = %v, want ErrUnavailable", bad, err)
		}
	}
}

func TestASheetIsOnlyOfferedOnceTheManifestIsWritten(t *testing.T) {
	// The manifest is written after the picture, so a half-built sheet is never
	// advertised. This pins the read side of that ordering.
	m := newTestManager(t, &absentFFmpeg{})
	key := Key("/films/heat.mkv", 1, 1)

	if err := os.WriteFile(filepath.Join(m.dir, key+".jpg"), []byte("not really a jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := m.Lookup(t.Context(), key, "/films/heat.mkv", 9000); !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v, want ErrUnavailable: a sheet with no manifest was offered", err)
	}
}

func TestAFileThatCannotBeBuiltIsNotRetriedForever(t *testing.T) {
	// Observed for real: ffmpeg refused an output filename it could not choose
	// a muxer for, and a polling player asked three times in a third of a
	// second, starting three encodes of the same unbuildable file.
	m := newTestManager(t, &absentFFmpeg{})
	key := Key("/films/broken.mkv", 1, 1)

	m.mu.Lock()
	m.failed[key] = true
	m.mu.Unlock()

	m.start(key, "/films/broken.mkv", 9000)

	m.mu.Lock()
	building := m.building[key]
	m.mu.Unlock()
	if building {
		t.Error("a build was started for a file already known to fail")
	}
}
