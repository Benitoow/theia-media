package playback

import (
	"os"
	"testing"

	"github.com/Benitoow/theia-media/internal/fakeffmpeg"
)

// TestMain lets this test binary double as a fake ffmpeg. See the
// fakeffmpeg package: in a normal run this returns immediately.
func TestMain(m *testing.M) {
	fakeffmpeg.MaybeRun()
	os.Exit(m.Run())
}
