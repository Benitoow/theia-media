// Package fakeffmpeg is a stand-in ffmpeg binary for tests that must hold a
// live converted stream without a real encoder on the machine.
//
// The real Manager cannot be faked at that seat: it re-hashes any binary it is
// given and downloads one when none is there, which is exactly what a unit
// test must never trigger. Instead the test binary doubles as the fake, re-
// executing itself. A TestMain calls MaybeRun first; when the mode variable is
// set, this process IS the fake, the arguments -- the real ffmpeg's -- are
// ignored without ever being parsed, and MaybeRun does not return.
package fakeffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
)

// The environment a test sets before the service spawns its children.
const (
	// ModeEnv turns the process into the fake. Its value selects behaviour.
	ModeEnv = "THEIA_FAKE_FFMPEG"

	// MarkerEnv names a directory where the fake records that it is running:
	// one file per process, named by its pid, removed only by a clean exit.
	// A marker that survives means the process was killed rather than ended.
	MarkerEnv = "THEIA_FAKE_FFMPEG_MARKER"
)

// Behaviours.
const (
	// ModeLive emits a chunk and then holds the stream open until killed.
	ModeLive = "live"

	// ModeBytes emits a chunk and exits cleanly.
	ModeBytes = "bytes"

	// ModeFail exits without emitting anything, the shape of an encoder that
	// never opens.
	ModeFail = "fail"
)

var chunk = bytes.Repeat([]byte{0x00}, 32<<10)

// MaybeRun is the first call of a test package's TestMain. It returns at once
// in a normal test run and never returns in a fake.
func MaybeRun() {
	mode := os.Getenv(ModeEnv)
	if mode == "" {
		return
	}
	marker := markerPath()
	switch mode {
	case ModeLive:
		// The chunk goes out before the marker: a test that waits for the
		// marker therefore waits for a first byte the pipe already holds, and
		// the peek that opens the response cannot miss it.
		_, _ = os.Stdout.Write(chunk)
		if marker != "" {
			if err := os.MkdirAll(filepath.Dir(marker), 0o755); err == nil {
				_ = os.WriteFile(marker, []byte("running"), 0o644)
			}
		}
		time.Sleep(10 * time.Minute) // killed, or the test times out first
	case ModeBytes:
		_, _ = os.Stdout.Write(chunk)
		if marker != "" {
			_ = os.Remove(marker)
		}
		os.Exit(0)
	case ModeFail:
		os.Exit(1)
	}
	os.Exit(0)
}

func markerPath() string {
	dir := os.Getenv(MarkerEnv)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("%d.pid", os.Getpid()))
}

// SetMode selects the behaviour of every fake the test spawns from now on.
func SetMode(mode, markerDir string) (restore func()) {
	previousMode, previousMarker := os.Getenv(ModeEnv), os.Getenv(MarkerEnv)
	os.Setenv(ModeEnv, mode)
	os.Setenv(MarkerEnv, markerDir)
	return func() {
		os.Setenv(ModeEnv, previousMode)
		os.Setenv(MarkerEnv, previousMarker)
	}
}

// Path is what the fake Binary reports as the encoder: this process itself,
// which MaybeRun turns back into the fake.
func Path() string { return os.Args[0] }

// Binary implements the delivery service's binary interface against this test
// binary. Playback.Binary is structural: nothing here imports it.
type Binary struct {
	// PathErr, when set, makes Path fail -- the ffmpeg_unavailable answer.
	PathErr error

	// Caps is what Capabilities reports.
	Caps ffmpeg.Capabilities

	// HWDecoder travels into the transcode arguments.
	HWDecoder string
}

func (b *Binary) Available() bool { return b.PathErr == nil }

func (b *Binary) Path(_ context.Context) (string, error) {
	if b.PathErr != nil {
		return "", b.PathErr
	}
	return Path(), nil
}

func (b *Binary) Capabilities(_ context.Context) ffmpeg.Capabilities { return b.Caps }

func (b *Binary) HardwareDecoder(_ context.Context) string { return b.HWDecoder }

// UsableEncoder answers the capabilities a transcode test needs: one encoder
// that runs, named for the log rather than for any real product.
func UsableEncoder() ffmpeg.Capabilities {
	return ffmpeg.Capabilities{Encoders: []ffmpeg.Encoder{{
		Name: "fakehw", Kind: ffmpeg.KindHardware, Vendor: "test",
	}}}
}
