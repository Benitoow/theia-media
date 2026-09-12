package playback

import (
	"sync"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
)

// transcodeLimiter keeps the furnace from being lit twice.
//
// The numbers come from a measurement rather than a feeling: on the
// maintainer's machine a 1080p HEVC source re-encodes at 1.04x real time in
// software and 4.56x on the GPU. One software transcode therefore consumes the
// whole margin, and a second would leave both viewers watching a spinner --
// the failure mode where nobody can tell what went wrong. Hardware has room
// for a few.
type transcodeLimiter struct {
	mu     sync.Mutex
	active int
	limit  int
}

func newTranscodeLimiter() *transcodeLimiter {
	return &transcodeLimiter{limit: 1}
}

// setKind raises the ceiling once the encoder is known.
func (l *transcodeLimiter) setKind(kind ffmpeg.EncoderKind) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if kind == ffmpeg.KindHardware {
		l.limit = 3
	} else {
		l.limit = 1
	}
}

// acquire takes a slot, returning the release function and whether it got one.
//
// toneMap is the second thing the ceiling has to know, and decision 87 measured
// why. Converting HDR to SDR is zscale work on the CPU: a GPU encoder does not
// relieve any of it, so a tone-mapped transcode runs at 1.09x real time at the
// source's own size whatever encoder is chosen -- the same margin the software
// limit of one exists to protect. Counting it as one of three hardware slots was
// optimistic, and two concurrent HDR playbacks would have stalled together with
// nobody able to say why.
//
// So it costs the whole budget: one runs, and anything else is refused with a
// code the interface can explain, which is the answer decision 58 chose over a
// queue nobody can see.
func (l *transcodeLimiter) acquire(toneMap bool) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cost := 1
	if toneMap {
		cost = l.limit
	}
	if l.active+cost > l.limit {
		return nil, false
	}
	l.active += cost
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.active -= cost
	}, true
}

func (l *transcodeLimiter) busy() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active >= l.limit
}
