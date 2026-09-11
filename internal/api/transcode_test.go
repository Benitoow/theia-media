package api

import (
	"testing"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
)

// Hardware has room for a few ordinary transcodes.
func TestHardwareRunsSeveralOrdinaryTranscodes(t *testing.T) {
	l := newTranscodeLimiter()
	l.setKind(ffmpeg.KindHardware)

	var releases []func()
	for i := 1; i <= 3; i++ {
		release, got := l.acquire(false)
		if !got {
			t.Fatalf("transcode %d was refused a hardware slot", i)
		}
		releases = append(releases, release)
	}
	if _, got := l.acquire(false); got {
		t.Error("a fourth transcode got a slot, and there are three")
	}
	for _, release := range releases {
		release()
	}
	if _, got := l.acquire(false); !got {
		t.Error("slots were not given back")
	}
}

// A tone-mapped one does not, because the work a GPU cannot take is the work
// that matters. Decision 87 measured 1.09x real time at source resolution.
func TestAToneMappedTranscodeTakesTheWholeBudget(t *testing.T) {
	l := newTranscodeLimiter()
	l.setKind(ffmpeg.KindHardware)

	release, got := l.acquire(true)
	if !got {
		t.Fatal("the first HDR transcode was refused")
	}
	if _, got := l.acquire(true); got {
		t.Error("a second HDR transcode got a slot; both would have stalled")
	}
	if _, got := l.acquire(false); got {
		t.Error("an ordinary transcode ran alongside an HDR one")
	}

	release()
	if _, got := l.acquire(false); !got {
		t.Error("the budget was not given back in full")
	}
}

// And it cannot slip in beside transcodes already running.
func TestAnHDRTranscodeWaitsForAQuietMachine(t *testing.T) {
	l := newTranscodeLimiter()
	l.setKind(ffmpeg.KindHardware)

	release, _ := l.acquire(false)
	if _, got := l.acquire(true); got {
		t.Error("an HDR transcode started while another was running")
	}
	release()
	if _, got := l.acquire(true); !got {
		t.Error("an HDR transcode was refused a machine with nothing on it")
	}
}

// In software there was only ever one slot, and tone mapping changes nothing.
func TestSoftwareStillRunsOneAtATime(t *testing.T) {
	l := newTranscodeLimiter()
	l.setKind(ffmpeg.KindSoftware)

	if _, got := l.acquire(false); !got {
		t.Fatal("the first software transcode was refused")
	}
	if _, got := l.acquire(false); got {
		t.Error("two software transcodes ran at once")
	}
}
