package playback

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

// SessionKind separates the two ceilings a converted stream sits under. A
// transcode is already governed by the transcode budget -- one software slot,
// a few hardware ones -- so it reserves only for the shutdown kill. A remux
// has no budget of its own: without the registry it would be N concurrent
// ffmpeg processes for N viewers, and the shutdown kill would have nothing to
// walk.
type SessionKind string

const (
	SessionRemux     SessionKind = "remux"
	SessionTranscode SessionKind = "transcode"
)

// DefaultStreamLimit is the remux ceiling. Four simultaneous rewraps is far
// above any real household and far below an embollement; the value is a
// decision, recorded with the code that enforces it, not a measurement.
const DefaultStreamLimit = 4

// Slot is one reserved place for a converted stream, between the decision to
// spawn a process and the moment that process exists.
//
// The two-phase shape exists so the ceiling can refuse before anything is
// spawned: refusing after the start would light an ffmpeg only to kill it a
// millisecond later, and log a stream that never delivered a byte.
type Slot struct {
	sessions *Sessions
	id       uint64

	mu       sync.Mutex
	proc     *os.Process
	kind     SessionKind
	identity string
	started  time.Time
}

// Attach records the process once it exists. A slot that never gets one --
// the start failed -- is simply released.
func (sl *Slot) Attach(proc *os.Process) {
	sl.mu.Lock()
	sl.proc = proc
	sl.mu.Unlock()
}

// Release gives the slot back. Once the process is attached, releasing means
// the handler has reaped it -- the copy loop ended, one way or another.
func (sl *Slot) Release() {
	sl.sessions.release(sl)
}

// Sessions tracks every converted stream Theia is currently feeding.
//
// Two jobs, both born of the same omission: the playback path used to spawn
// ffmpeg with no memory of it. Nothing capped how many remuxes could run at
// once, and every exit path that skips its deferred calls -- the updater's
// os.Exit above all -- left a film's encoder running on a machine whose server
// had already gone away.
//
// The graceful HTTP drain stays the first line of shutdown; killing the
// tracked children is the net that makes the exits that skip defers harmless.
// Process.Kill on the tracked commands is enough on all three platforms, with
// no Job Objects and no cgo.
type Sessions struct {
	log *slog.Logger

	mu          sync.Mutex
	nextID      uint64
	live        map[uint64]*Slot
	remuxActive int
	limit       int
}

// NewSessions prepares the registry. limit is the remux ceiling; zero selects
// DefaultStreamLimit.
func NewSessions(limit int, log *slog.Logger) *Sessions {
	if limit <= 0 {
		limit = DefaultStreamLimit
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Sessions{
		log:   log,
		live:  map[uint64]*Slot{},
		limit: limit,
	}
}

// Acquire reserves a slot for a converted stream of the given kind.
//
// The reservation is unconditional for a transcode -- its real budget is the
// transcode limiter -- and subject to the remux ceiling otherwise. A nil slot
// therefore only ever means "the remux ceiling is reached", and the caller
// answers with the one code the interface already retries.
func (s *Sessions) Acquire(kind SessionKind, identity string) *Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if kind == SessionRemux && s.remuxActive >= s.limit {
		return nil
	}
	s.nextID++
	id := s.nextID
	if kind == SessionRemux {
		s.remuxActive++
	}
	slot := &Slot{
		sessions: s,
		id:       id,
		kind:     kind,
		identity: identity,
		started:  time.Now(),
	}
	// Unattached: the process does not exist yet, and KillAll must not meet a
	// nil process. It walks live slots with a process only.
	s.live[id] = slot
	return slot
}

func (s *Sessions) release(slot *Slot) {
	s.mu.Lock()
	if _, ok := s.live[slot.id]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.live, slot.id)
	if slot.kind == SessionRemux {
		s.remuxActive--
	}
	s.mu.Unlock()
}

// KillAll terminates every live stream and reports how many it had to kill.
//
// It does not wait. The handler that owns each process reaps it on the way out
// of its own copy loop; a process that ignores the kill loses its stdout pipe
// the moment it dies, which is what unblocks the copy either way.
func (s *Sessions) KillAll() int {
	s.mu.Lock()
	live := s.live
	s.live = map[uint64]*Slot{}
	s.remuxActive = 0
	s.mu.Unlock()

	killed := 0
	for id, slot := range live {
		slot.mu.Lock()
		proc := slot.proc
		slot.mu.Unlock()
		if proc == nil {
			continue
		}
		if err := proc.Kill(); err != nil {
			s.log.Warn("killing a live media stream failed", "id", id,
				"kind", slot.kind, "identity", slot.identity, "error", err)
			continue
		}
		killed++
		s.log.Info("killed a live media stream", "id", id, "kind", slot.kind,
			"identity", slot.identity, "age_seconds", time.Since(slot.started).Seconds())
	}
	return killed
}

// Active reports how many converted streams are live, whatever their kind.
// Support diagnostics and the tests both read it; nothing in the request path
// does.
func (s *Sessions) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.live)
}

// RemuxActive reports how many remux slots are taken. It is the number the
// ceiling governs.
func (s *Sessions) RemuxActive() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.remuxActive
}
