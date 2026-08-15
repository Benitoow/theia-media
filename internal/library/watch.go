package library

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/scanner"
)

// Watch intervals.
//
// interval is how often the disk is looked at. It is a compromise, and the
// thing being traded is not CPU: a stat-only walk of a few thousand entries
// costs nothing. It is the external drive that has spun down. Looking every ten
// seconds would keep every disk in the house awake forever to notice a film
// that arrives once a fortnight.
//
// stability is how recently a file may have been written and still be trusted.
// A file arriving over the network is not there all at once, and indexing it
// halfway produces a card with no duration, a failed inspection and a wasted
// TMDB lookup, all of which have to be undone on the next pass. Anything
// touched inside this window is left out of the reckoning entirely, which means
// a copy in progress reads as "nothing has changed yet" rather than as a new
// film -- and the pass after it finishes sees it properly.
const (
	DefaultWatchInterval  = 60 * time.Second
	defaultStabilityDelay = 15 * time.Second
)

// quiet swallows the log output of the observation walk.
//
// The walk runs every interval forever, and a drive that is unplugged is not an
// event -- it is a Tuesday, and the scanner says so once per root per walk. At
// one pass a minute that is fourteen hundred identical warnings a day, which
// would bury the one line that matters. The reconciling scan that follows a
// real change keeps the ordinary logger, so nothing genuinely new goes unsaid.
var quiet = slog.New(slog.DiscardHandler)

// Watcher keeps the library in step with the disk without anybody asking.
//
// It does not subscribe to filesystem events. That was considered and rejected:
// inotify and ReadDirectoryChangesW do not cross an SMB share, and the scanner
// package opens by saying that a real library lives on external drives and
// network shares. A notifier that silently delivers nothing on exactly the
// storage this project expects would be worse than no notifier at all, because
// it would look like it worked. So the disk is read instead, cheaply, and the
// answer is compared with the last one.
//
// What makes that affordable is that reading the disk and reconciling with the
// database are two different costs. The walk is stat calls; the reconciliation
// is thousands of writes, a consolidation pass and a queue of TMDB lookups.
// Only the first happens on a quiet minute.
type Watcher struct {
	svc *Service
	log *slog.Logger

	interval  time.Duration
	stability time.Duration

	mu    sync.Mutex
	roots []string

	// wake asks for an immediate pass. Buffered, and written to without
	// blocking: two folder changes a second apart owe us one scan, not two.
	wake chan struct{}

	// seen is the fingerprint of the settled files as of the last successful
	// reconciliation, and is touched only by the goroutine running Run.
	seen    uint64
	hasSeen bool
}

// NewWatcher builds a watcher over the given roots. It does nothing until Run
// is called.
func NewWatcher(svc *Service, roots []string, log *slog.Logger) *Watcher {
	return &Watcher{
		svc:       svc,
		log:       log,
		interval:  DefaultWatchInterval,
		stability: defaultStabilityDelay,
		roots:     slices.Clone(roots),
		wake:      make(chan struct{}, 1),
	}
}

// Roots returns the directories currently being watched.
//
// The watcher owns this list rather than reading it back out of the
// configuration, because the settings handler and this goroutine would
// otherwise be touching the same slice from two threads.
func (w *Watcher) Roots() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.roots)
}

// SetRoots replaces the watched directories. It does not itself trigger a pass;
// callers that changed the folders on purpose should follow with Wake.
func (w *Watcher) SetRoots(roots []string) {
	w.mu.Lock()
	w.roots = slices.Clone(roots)
	w.mu.Unlock()
}

// Wake asks for a reconciliation now, whether or not the disk looks different.
// Adding a folder in the settings should fill the library while the user is
// still looking at the page, not a minute after they have gone.
func (w *Watcher) Wake() {
	select {
	case w.wake <- struct{}{}:
	default: // a pass is already pending; it will see everything this one would
	}
}

// Run reconciles once immediately, then keeps watching until ctx is cancelled.
//
// The first pass is the startup scan: it is forced, because an empty
// fingerprint would otherwise make the first tick do the same work a minute
// later and call it a change.
func (w *Watcher) Run(ctx context.Context) {
	w.pass(ctx, true)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
			w.pass(ctx, true)
		case <-ticker.C:
			w.pass(ctx, false)
		}
	}
}

// pass looks at the disk and reconciles if there is a reason to.
func (w *Watcher) pass(ctx context.Context, force bool) {
	roots := w.Roots()
	if len(roots) == 0 {
		return
	}

	fingerprint, ok := w.observe(ctx, roots)
	if !ok {
		return
	}

	if !force && w.hasSeen && fingerprint == w.seen && !w.svc.HasOutstandingMetadata(ctx) {
		return
	}

	report, err := w.svc.Scan(ctx, roots)
	switch {
	case errors.Is(err, ErrScanInProgress):
		// Somebody pressed the button, or a previous pass is still going. Their
		// scan does exactly this work; claiming this fingerprint as reconciled
		// would be taking credit for it, so the next pass tries again.
		return
	case errors.Is(err, context.Canceled):
		return
	case err != nil:
		w.log.Error("the automatic scan failed", "error", err)
		return
	}

	w.seen, w.hasSeen = fingerprint, true

	if report.Added > 0 || report.Removed > 0 {
		w.log.Info("the library changed on disk",
			"added", report.Added, "removed", report.Removed, "found", report.Found)
	}
}

// observe walks the roots and reduces what it saw to one number.
//
// Files written within the stability window are left out, so that a copy in
// progress cannot change the answer until it has finished. Problems are folded
// in as well: a root that has vanished takes its files with it, and without
// this a library whose every file lived on that one drive would produce the
// same fingerprint as a library that had genuinely been emptied.
func (w *Watcher) observe(ctx context.Context, roots []string) (uint64, bool) {
	result, err := scanner.Scan(ctx, roots, quiet)
	if err != nil {
		return 0, false
	}

	settled := time.Now().Add(-w.stability)
	digest := fnv.New64a()
	scratch := make([]byte, 8)

	number := func(v int64) {
		binary.LittleEndian.PutUint64(scratch, uint64(v))
		_, _ = digest.Write(scratch)
	}
	text := func(s string) {
		_, _ = digest.Write([]byte(s))
		_, _ = digest.Write([]byte{0})
	}

	for _, file := range result.Files {
		if file.ModifiedAt.After(settled) {
			continue
		}
		text(file.Path)
		number(file.SizeBytes)
		number(file.ModifiedAt.UnixNano())
	}
	for _, problem := range result.Problems {
		text(string(problem.Kind))
		text(problem.Path)
	}
	return digest.Sum64(), true
}
