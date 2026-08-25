package preview

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
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

	_, err := m.Lookup(t.Context(), Key("/films/heat.mkv", 100, 7), "/films/heat.mkv", 9000, "")
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

	if _, err := m.Lookup(t.Context(), key, "/films/short.mkv", 30, ""); !errors.Is(err, ErrUnavailable) {
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

	if _, err := m.Lookup(t.Context(), key, "/films/heat.mkv", 9000, ""); !errors.Is(err, ErrUnavailable) {
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

	m.start(key, "/films/broken.mkv", 9000, "")

	m.mu.Lock()
	building := m.building[key]
	m.mu.Unlock()
	if building {
		t.Error("a build was started for a file already known to fail")
	}
}

// The build had no end at all, and one encode runs at a time: a file ffmpeg
// could not finish held that slot for the life of the process and no other film
// ever got a strip.
func TestBuildTimeoutIsSizedFromTheFilm(t *testing.T) {
	cases := []struct {
		name     string
		duration float64
		want     time.Duration
	}{
		// The file this was measured against: 2 h 35, and a real build of 217 s.
		// The allowance has to clear that with room, and does -- 466 s.
		{"a 4K feature", 9326.61, 466 * time.Second},
		// Short films are not proportionally cheaper: the cost is reading the
		// file, not its length.
		{"a short", 300, 2 * time.Minute},
		{"the shortest thing worth a strip", 121, 2 * time.Minute},
		// And past a quarter of an hour the slot is worth more to the next film.
		{"something absurd", 60 * 60 * 20, 15 * time.Minute},
	}
	for _, c := range cases {
		if got := buildTimeout(c.duration); got != c.want {
			t.Errorf("%s (%.0fs): timeout = %v, want %v", c.name, c.duration, got, c.want)
		}
	}
}

// The measured build must sit comfortably inside its own allowance, or the
// timeout is a way of never producing a strip for a real film.
func TestTheMeasuredBuildFitsInsideItsAllowance(t *testing.T) {
	const measured = 217 * time.Second // the 4K HDR file, timed
	allowed := buildTimeout(9326.61)
	if allowed < 2*measured {
		t.Errorf("allowance %v leaves less than double the measured %v", allowed, measured)
	}
}
