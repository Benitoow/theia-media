// Package updater keeps Theia up to date from its GitHub releases.
//
// The governing rule is that a failed update must leave a working installation.
// Every step that could fail happens before anything irreversible: the binary is
// downloaded to a temporary file, its digest checked, and the file actually
// executed to confirm it runs and reports the version it claims -- all before
// the running executable is touched. The swap itself is two renames with a
// rollback if the second one fails.
//
// The second rule is that an update never interrupts playback. A film cut short
// by an improvement is worse than an improvement that waits.
package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/activity"
)

// State is what the updater is doing, as reported to the interface.
type State string

const (
	StateIdle        State = "idle"        // nothing to do, or never checked
	StateChecking    State = "checking"    // asking GitHub
	StateAvailable   State = "available"   // a newer version exists
	StateDownloading State = "downloading" // fetching and verifying
	StateReady       State = "ready"       // swapped; a restart will run it
	StateDeferred    State = "deferred"    // held back because something is playing
	StateFailed      State = "failed"      // the current binary is untouched
	StateUnsupported State = "unsupported" // a development build cannot update itself
)

// Status is the full picture, safe to serialise straight to the interface.
type Status struct {
	State          State      `json:"state"`
	CurrentVersion string     `json:"current_version"`
	LatestVersion  string     `json:"latest_version,omitempty"`
	Available      bool       `json:"available"`
	Message        string     `json:"message,omitempty"`
	ReleaseURL     string     `json:"release_url,omitempty"`
	CheckedAt      *time.Time `json:"checked_at,omitempty"`
}

// checkInterval is how often the updater asks GitHub on its own.
const checkInterval = 6 * time.Hour

// Updater checks for and applies new versions.
type Updater struct {
	repo     string
	current  string
	apiBase  string
	http     *http.Client
	log      *slog.Logger
	activity *activity.Tracker

	// restart is called once a new binary is in place. It is main's job: close
	// the listener, start the replacement, exit.
	restart func()

	// execPath is the file to replace. Injected so tests can point it at a
	// copy rather than at the running test binary.
	execPath string

	mu     sync.Mutex
	status Status
	busy   bool
}

// Options configures an Updater.
type Options struct {
	Repo     string
	Version  string
	ExecPath string
	Activity *activity.Tracker
	Logger   *slog.Logger
	Restart  func()

	// APIBase defaults to GitHub. Tests point it at a stub.
	APIBase string
}

// New builds an Updater.
func New(opts Options) *Updater {
	apiBase := opts.APIBase
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}

	u := &Updater{
		repo:     opts.Repo,
		current:  opts.Version,
		apiBase:  apiBase,
		http:     &http.Client{Timeout: 10 * time.Minute},
		log:      opts.Logger,
		activity: opts.Activity,
		restart:  opts.Restart,
		execPath: opts.ExecPath,
		status:   Status{State: StateIdle, CurrentVersion: opts.Version},
	}

	// A build whose version cannot be parsed -- "dev", a bare commit hash --
	// must never replace itself. There is nothing to compare against, so any
	// answer would be a guess, and the guess would overwrite a developer's
	// working binary.
	if _, ok := parseVersion(opts.Version); !ok {
		u.status.State = StateUnsupported
		u.status.Message = "this is a development build and does not update itself"
	}
	return u
}

// Status returns the current picture.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

// Supported reports whether this build can update itself at all.
func (u *Updater) Supported() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status.State != StateUnsupported
}

func (u *Updater) setStatus(mutate func(*Status)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	mutate(&u.status)
}

// Check asks GitHub whether a newer version exists. It never changes anything
// on disk.
func (u *Updater) Check(ctx context.Context) (Status, error) {
	if !u.Supported() {
		return u.Status(), nil
	}

	u.setStatus(func(s *Status) { s.State = StateChecking })

	rel, err := u.latestRelease(ctx)
	now := time.Now().UTC()

	if errors.Is(err, ErrNoRelease) {
		u.setStatus(func(s *Status) {
			s.State = StateIdle
			s.Available = false
			s.Message = "no release has been published yet"
			s.CheckedAt = &now
		})
		return u.Status(), nil
	}
	if err != nil {
		u.log.Warn("checking for updates failed", "error", err)
		u.setStatus(func(s *Status) {
			s.State = StateIdle
			s.Message = "could not reach GitHub to check for updates"
			s.CheckedAt = &now
		})
		return u.Status(), err
	}

	newer, comparable := isNewer(rel.TagName, u.current)
	u.setStatus(func(s *Status) {
		s.CheckedAt = &now
		s.LatestVersion = rel.TagName
		s.ReleaseURL = rel.HTMLURL
		s.Available = newer
		switch {
		case !comparable:
			s.State = StateUnsupported
			s.Message = "the published version cannot be compared with this build"
		case newer:
			s.State = StateAvailable
			s.Message = ""
		default:
			s.State = StateIdle
			s.Message = "this is the latest version"
		}
	})

	if newer {
		u.log.Info("an update is available", "current", u.current, "latest", rel.TagName)
	}
	return u.Status(), nil
}

// Run performs a check now and then every few hours, until the context ends.
//
// It only ever checks. Applying is a separate, explicit step, because a media
// server that restarts itself unannounced is a media server nobody trusts.
func (u *Updater) Run(ctx context.Context) {
	if !u.Supported() {
		u.log.Info("automatic updates are off for this build", "version", u.current)
		return
	}

	if _, err := u.Check(ctx); err != nil && !errors.Is(err, context.Canceled) {
		u.log.Debug("the initial update check did not complete", "error", err)
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := u.Check(ctx); err != nil && !errors.Is(err, context.Canceled) {
				u.log.Debug("an update check did not complete", "error", err)
			}
		}
	}
}

// ErrBusy is returned when an update is already running.
var ErrBusy = errors.New("updater: an update is already in progress")

// ErrPlaybackInProgress is returned when something is being watched.
var ErrPlaybackInProgress = errors.New("updater: something is playing")

// claim marks the updater busy, or reports that it already was.
func (u *Updater) claim() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.busy {
		return ErrBusy
	}
	u.busy = true
	return nil
}

func (u *Updater) release() {
	u.mu.Lock()
	u.busy = false
	u.mu.Unlock()
}

func (u *Updater) fail(err error, message string) error {
	u.setStatus(func(s *Status) {
		s.State = StateFailed
		s.Message = message
	})
	u.log.Error("the update was abandoned; the installed version is unchanged",
		"error", err, "reason", message)
	return fmt.Errorf("%s: %w", message, err)
}
