// Package activity tracks whether anybody is currently watching something.
//
// It exists for one reason: an update must never interrupt playback. A film
// cutting out because the server decided to restart is the worst possible
// moment for an improvement to arrive.
package activity

import (
	"sync"
	"time"
)

// idleAfter is how long playback is assumed to continue after the last request.
//
// The two playback paths look completely different from the server's side. A
// remuxed stream is one request held open for the length of the film, so the
// in-flight count alone would catch it. Direct play is a burst of short range
// requests with gaps of many seconds between them, so the count is usually zero
// mid-film. Only the combination of the two signals describes both.
const idleAfter = 90 * time.Second

// Tracker records stream activity.
type Tracker struct {
	mu       sync.Mutex
	inFlight int
	last     time.Time
}

// New returns a tracker with nothing in progress.
func New() *Tracker { return &Tracker{} }

// Begin records the start of a stream request and returns the function to call
// when it ends.
func (t *Tracker) Begin() func() {
	t.mu.Lock()
	t.inFlight++
	t.last = time.Now()
	t.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			t.inFlight--
			t.last = time.Now()
			t.mu.Unlock()
		})
	}
}

// Busy reports whether something is being watched right now.
func (t *Tracker) Busy() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.inFlight > 0 || (!t.last.IsZero() && time.Since(t.last) < idleAfter)
}

// IdleFor returns how long nothing has been streamed. Zero while a stream is
// open, and a very large duration when nothing has ever played.
func (t *Tracker) IdleFor() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inFlight > 0 {
		return 0
	}
	if t.last.IsZero() {
		// Nothing has played since the server started. That is idle, not
		// unknown, and it must not read as "zero seconds idle".
		return 1<<62 - 1
	}
	return time.Since(t.last)
}
