package setup

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Benitoow/theia-media/internal/release"
	"github.com/Benitoow/theia-media/internal/updater"
)

// previousBundleSuffix names the file an update moved aside.
//
// The player has no updater of its own: the server updates itself, and the
// player was refreshed by re-running the installer, which is a thing people do
// once. This is the missing half - and it is here rather than inside the player
// because replacing a running program from within itself is the one update
// nobody can roll back.
const previousBundleSuffix = ".previous"

// ErrPlayerNotInstalled says there is nothing to update. It is separate from the
// other failures because it is the one a person can act on without any
// technical detail: install the player, then ask again.
var ErrPlayerNotInstalled = errors.New("setup: no player is installed here")

// PlayerTarget is the installed player an update would replace.
type PlayerTarget struct {
	// Dir is the installation directory the bundle lives in.
	Dir string

	// ExecPath is the player executable on disk.
	ExecPath string

	// Version is what the installed player reports when it is asked. "dev" is
	// honest for a build made from a working tree: every release is newer than
	// it, and decision 24 refuses to update a build that cannot name itself.
	Version string
}

// ResolvePlayerTarget finds the installed player and asks it what version it
// carries rather than trusting the directory.
//
// The player had no way to answer that until this work: it printed its version
// only from inside a running window, which is not an answer a tool can read.
// `theia-player -version` now prints it and exits before anything is created.
//
// An explicit directory wins over the machine's own, which is what `--install-dir`
// means everywhere else in this tool, and what lets the whole path be driven
// against a throwaway installation instead of the one somebody is using.
func ResolvePlayerTarget(installDir string) (PlayerTarget, error) {
	dir := installDir
	if strings.TrimSpace(dir) == "" {
		standard, err := DefaultInstallDir()
		if err != nil {
			return PlayerTarget{}, err
		}
		dir = standard
	}
	path := filepath.Join(dir, playerExecutablePath(runtime.GOOS))
	if !fileExists(path) {
		return PlayerTarget{}, fmt.Errorf("%w: %s", ErrPlayerNotInstalled, path)
	}
	version, err := binaryVersion(path)
	if err != nil {
		// A player built before decision 143 cannot answer: `-version` did not
		// exist, so the argument arrived as noise and the program tried to start.
		// Those installations are exactly the ones this path exists for, so the
		// version is taken from the server in the same directory - the two ship in
		// one release, and this is the only moment their versions can differ in a
		// way nothing can see. Asking the player is still what happens first, and
		// what happens afterwards for every installation the update has touched.
		sibling, siblingErr := serverVersionBeside(dir)
		if siblingErr != nil {
			return PlayerTarget{}, err
		}
		return PlayerTarget{Dir: dir, ExecPath: path, Version: sibling}, nil
	}
	return PlayerTarget{Dir: dir, ExecPath: path, Version: version}, nil
}

// serverVersionBeside asks the server installed next to the player what version
// it carries. It is a fallback with one use, described above.
func serverVersionBeside(dir string) (string, error) {
	path := firstInstalled(dir, "theia-server")
	if path == "" {
		return "", fmt.Errorf("setup: no server beside the player in %s", dir)
	}
	return binaryVersion(path)
}

// playerExecutablePath is the file that answers `-version` inside an
// installation, relative to its directory.
//
// On Windows and Linux the player is a program beside its engine. On macOS it is
// an app bundle - which is what gives a window an identity and a Dock icon - and
// the file a shell can run is the one inside Contents/MacOS. This is the file the
// update path asks for a version, and the one a running player holds open.
func playerExecutablePath(goos string) string {
	if goos == "darwin" {
		return path.Join("Theia.app", "Contents", "MacOS", "theia-player")
	}
	if goos == "windows" {
		return "theia-player.exe"
	}
	return "theia-player"
}

// playerBundleMembers is what an installation of the player consists of, as
// paths relative to the installation directory.
//
// The list itself lives in install.go, beside the code that asks the archive for
// it and checks an installation against it: one place, because a bundle missing
// its engine or its licence looks installed and is not.
func playerBundleMembers(goos string) []string {
	return bundleFiles(goos)
}

// swapMembers reduces a member list to what a replacement has to move: a member
// that contains others replaces them all, and moving it aside once is also the
// only way a directory can be replaced. On macOS that leaves `Theia.app` alone;
// on Windows it changes nothing, because no member contains another.
func swapMembers(names []string) []string {
	kept := make([]string, 0, len(names))
	for _, name := range names {
		contained := false
		for _, other := range names {
			if other == name {
				continue
			}
			if strings.HasPrefix(name, strings.TrimSuffix(other, "/")+"/") {
				contained = true
				break
			}
		}
		if !contained {
			kept = append(kept, name)
		}
	}
	return kept
}

// CheckForPlayerUpdate asks GitHub Releases for the latest player. Nothing is
// downloaded and nothing on disk is touched.
func CheckForPlayerUpdate(ctx context.Context, client *http.Client, apiBase string, target PlayerTarget) (UpdateStatusView, error) {
	if !updater.Comparable(target.Version) {
		return uncheckedPlayerView(target), nil
	}
	rel, _, err := latestPlayerRelease(ctx, client, apiBase)
	if err != nil {
		return failedPlayerView(target, err), unreachable(err)
	}
	return checkedPlayerView(target, rel), nil
}

// ApplyPlayerUpdate downloads the bundle the release publishes, verifies its
// digest, proves the player inside it starts and names itself, and only then
// touches the installation.
//
// The order is the same discipline the server's updater applies, and for the
// same reason: every step that can fail happens before anything irreversible. A
// bundle that fails its digest, that is missing its licence or whose executable
// does not run is refused with the installation untouched.
func ApplyPlayerUpdate(ctx context.Context, client *http.Client, apiBase string, target PlayerTarget, report Reporter) (UpdateStatusView, error) {
	if !updater.Comparable(target.Version) {
		return uncheckedPlayerView(target), nil
	}
	rel, asset, err := latestPlayerRelease(ctx, client, apiBase)
	if err != nil {
		return failedPlayerView(target, err), unreachable(err)
	}
	view := checkedPlayerView(target, rel)
	if !view.Available {
		return view, nil
	}

	names := playerBundleMembers(runtime.GOOS)
	if blocker, held := bundleBlocker(target.Dir, names); blocker != "" {
		// Nothing was downloaded and nothing was touched, so the answer is not
		// "an update is available": leaving that flag set is how this line once
		// printed "an update is available" and then said nothing else at all.
		view.State, view.Available = "failed", false
		if held {
			view.Reason = "player_in_use"
			view.Message = "the installed player holds " + blocker +
				"; closing it is the only way to replace it"
		} else {
			view.Reason = "replace_failed"
			view.Message = "the installation's " + blocker + " could not be opened for writing"
		}
		return view, nil
	}

	if err := fetchAndSwap(ctx, target, asset, rel.Tag, names, report); err != nil {
		reason, message := playerReason(err)
		view.State, view.Reason, view.Message, view.Available = "failed", reason, message, false
		return view, err
	}

	view.State, view.Reason, view.Available = "ready", "", false
	view.Latest = rel.Tag
	// What is installed now, asked of the installed player rather than assumed
	// from the release: the two agree, and a report that says so is the point.
	if installed, err := binaryVersion(target.ExecPath); err == nil {
		view.Current = installed
	} else {
		view.Current = rel.Tag
	}
	return view, nil
}

// unreachable decides whether a failure to ask GitHub is an error the caller
// must see or a state it should describe. "Nothing has been published yet" and
// "this release has no bundle for this machine" are facts about the release, not
// about the network, and both belong in a sentence rather than in a stack.
func unreachable(err error) error {
	if errors.Is(err, release.ErrNoRelease) || errors.Is(err, errNoPlayerAsset) {
		return nil
	}
	return err
}

// fetchAndSwap is the part that downloads and installs, kept separate so the
// decision above stays readable as a sequence of questions.
func fetchAndSwap(ctx context.Context, target PlayerTarget, asset release.Asset, tag string, names []string, report Reporter) error {
	staging, err := os.MkdirTemp("", "theia-player-update-")
	if err != nil {
		return fmt.Errorf("setup: creating a staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	phase(report, PhaseDownloading, "player")
	archive, err := asset.Download(ctx, nil, staging, func(done, total int64) { progress(report, done, total) })
	if err != nil {
		return errDownloadUnverified{err}
	}

	phase(report, PhaseExtracting, "player")
	if _, err := release.Extract(archive, staging, names); err != nil {
		return errBundleIncomplete{err}
	}

	staged := filepath.Join(staging, playerExecutablePath(runtime.GOOS))
	reported, err := binaryVersion(staged)
	if err != nil {
		return errPlayerDidNotRun{err}
	}
	if !sameVersion(reported, tag) {
		return errWrongVersion{reported: reported, tag: tag}
	}

	phase(report, PhaseInstalling, "player")
	// What is extracted and what is moved aside are not the same list: every
	// member has to come out of the archive, and the ones a bundle root contains
	// are replaced with it.
	if err := swapBundle(staging, target.Dir, swapMembers(names)); err != nil {
		return errReplaceFailed{err}
	}
	return nil
}

// swapBundle puts a staged bundle into the installation, one file at a time.
//
// Two renames per file, the shape the server's updater already uses for a single
// binary: the installed file is moved aside, the staged one takes its name, and
// anything that fails puts what was already moved back. Renaming rather than
// deleting is what lets this work while the player is open - a loaded library
// cannot be deleted, it can be renamed - and it is why the files moved aside are
// swept up by the next update instead of here, where Windows may still refuse.
func swapBundle(staged, dir string, names []string) error {
	moved := make([]string, 0, len(names))
	restore := func() {
		for _, name := range moved {
			target := filepath.Join(dir, name)
			os.Rename(target+previousBundleSuffix, target)
		}
	}
	for _, name := range names {
		target := filepath.Join(dir, name)
		aside := target + previousBundleSuffix
		// A leftover from an earlier update that could not delete it. Best
		// effort: the next successful update removes it.
		os.Remove(aside)
		if fileExists(target) {
			if err := os.Rename(target, aside); err != nil {
				restore()
				return fmt.Errorf("setup: moving %s aside: %w", name, err)
			}
		}
		if err := os.Rename(filepath.Join(staged, name), target); err != nil {
			os.Rename(aside, target)
			restore()
			return fmt.Errorf("setup: putting %s in place: %w", name, err)
		}
		moved = append(moved, name)
	}
	// Everything is in place, so what was moved aside is now dead weight - and
	// one of those files is the engine, a hundred megabytes of it. They are
	// removed here, best effort: a file a running program still holds refuses and
	// stays behind, and the next update removes it before it moves anything.
	for _, name := range moved {
		os.Remove(filepath.Join(dir, name+previousBundleSuffix))
	}
	return nil
}

// bundleBlocker names the first installed file that cannot be written yet, and
// whether that is because a running program holds it.
//
// It is a probe rather than a guess, and it happens before the download: an
// executing Windows image is opened with sharing that allows deletion but not
// writing, so an O_RDWR open fails exactly when something holds the file. On
// macOS the same rule applies to the Mach-O image inside the app bundle.
// Finding this out after a hundred megabytes have arrived would be a wasted
// minute and a worse sentence.
//
// A member that is a directory - the engine's own directory, on macOS - is
// walked: the file that matters is whichever library a running player has
// mapped, and a directory cannot be opened for writing at all.
func bundleBlocker(dir string, names []string) (name string, held bool) {
	for _, candidate := range names {
		path := filepath.Join(dir, candidate)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if blocked, isHeld := fileIsBlocked(filepath.Join(path, entry.Name())); blocked {
					return filepath.Join(candidate, entry.Name()), isHeld
				}
			}
			continue
		}
		if blocked, isHeld := fileIsBlocked(path); blocked {
			return candidate, isHeld
		}
	}
	return "", false
}

// fileIsBlocked reports whether a file cannot be opened for writing, and whether
// that is because something is using it.
func fileIsBlocked(path string) (blocked, held bool) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err == nil {
		file.Close()
		return false, false
	}
	return true, fileIsHeld(err)
}

// fileIsHeld reports whether Windows refused an open because something is using
// the file, rather than for any other reason.
//
// Only Windows can answer this, and only Windows needs to: a Unix program does
// not lock its own image, so replacing a running one is an ordinary rename
// there. The two numbers are ERROR_SHARING_VIOLATION and ERROR_LOCK_VIOLATION.
func fileIsHeld(err error) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	const sharingViolation, lockViolation = 32, 33
	return errno == sharingViolation || errno == lockViolation
}

// latestPlayerRelease asks for the release and the bundle this platform needs.
func latestPlayerRelease(ctx context.Context, client *http.Client, apiBase string) (release.Release, release.Asset, error) {
	rel, err := release.Latest(ctx, client, apiBase, release.DefaultRepo)
	if err != nil {
		return release.Release{}, release.Asset{}, err
	}
	asset, err := rel.Named(release.PlayerName(runtime.GOOS, runtime.GOARCH))
	if err != nil {
		return release.Release{}, release.Asset{}, fmt.Errorf("%w: %v", errNoPlayerAsset, err)
	}
	return rel, asset, nil
}

// uncheckedPlayerView is the answer for a build that cannot be compared with a
// release: nothing was asked, nothing was downloaded, nothing changed. Decision
// 24, applied to the second program.
func uncheckedPlayerView(target PlayerTarget) UpdateStatusView {
	return UpdateStatusView{
		State:    "failed",
		Current:  target.Version,
		Reason:   "development_build",
		ExecPath: target.ExecPath,
	}
}

// checkedPlayerView says what a check found, without changing anything.
func checkedPlayerView(target PlayerTarget, rel release.Release) UpdateStatusView {
	view := UpdateStatusView{
		State:    "idle",
		Current:  target.Version,
		Latest:   rel.Tag,
		ExecPath: target.ExecPath,
	}
	newer, comparable := updater.IsNewer(rel.Tag, target.Version)
	switch {
	case !comparable:
		// Decision 24, applied to the second program: a build that cannot say
		// what it is has no business being replaced by something it cannot be
		// compared against. A working tree builds `dev`.
		view.State, view.Reason = "failed", "development_build"
	case !newer:
		view.Reason = "up_to_date"
	default:
		view.Available = true
	}
	return view
}

// failedPlayerView turns a fetch failure into the code the interface turns into
// a sentence.
func failedPlayerView(target PlayerTarget, err error) UpdateStatusView {
	reason := "github_unreachable"
	switch {
	case errors.Is(err, release.ErrNoRelease):
		reason = "no_release"
	case errors.Is(err, errNoPlayerAsset):
		reason = "no_binary_for_platform"
	}
	return UpdateStatusView{
		State:    "failed",
		Current:  target.Version,
		Reason:   reason,
		Message:  err.Error(),
		ExecPath: target.ExecPath,
	}
}

// errNoPlayerAsset is what a release without this platform's bundle means, kept
// distinct from a release that could not be reached: one is a fact about the
// release, the other about the network.
var errNoPlayerAsset = errors.New("setup: this release publishes no player bundle for this machine")

// The refusals of the apply path, each one a sentence the interface owns the
// wording of.
type (
	errDownloadUnverified struct{ err error }
	errBundleIncomplete   struct{ err error }
	errPlayerDidNotRun    struct{ err error }
	errWrongVersion       struct{ reported, tag string }
	errReplaceFailed      struct{ err error }
)

func (e errDownloadUnverified) Error() string {
	return fmt.Sprintf("setup: the downloaded bundle was refused: %v", e.err)
}

func (e errBundleIncomplete) Error() string {
	return fmt.Sprintf("setup: the bundle is incomplete: %v", e.err)
}

func (e errPlayerDidNotRun) Error() string {
	return fmt.Sprintf("setup: the downloaded player did not run: %v", e.err)
}

func (e errWrongVersion) Error() string {
	return fmt.Sprintf("setup: the bundle carries %s where the release announces %s", e.reported, e.tag)
}

func (e errReplaceFailed) Error() string {
	return fmt.Sprintf("setup: replacing the player failed and the previous files were put back: %v", e.err)
}

// playerReason maps a refusal onto the code the terminal and the JSON report
// carry. Every one of them means the same thing to the installation: unchanged.
func playerReason(err error) (reason, message string) {
	var (
		unverified errDownloadUnverified
		incomplete errBundleIncomplete
		didNotRun  errPlayerDidNotRun
		wrong      errWrongVersion
		replaced   errReplaceFailed
	)
	switch {
	case errors.As(err, &unverified):
		return "download_not_verified", unverified.Error()
	case errors.As(err, &incomplete):
		return "bundle_incomplete", incomplete.Error()
	case errors.As(err, &didNotRun):
		return "binary_did_not_run", didNotRun.Error()
	case errors.As(err, &wrong):
		return "download_not_verified", wrong.Error()
	case errors.As(err, &replaced):
		return "replace_failed", replaced.Error()
	}
	return "replace_failed", err.Error()
}

// sameVersion compares a build's own version with the tag it came from. The "v"
// is the tag's, not the product's, so it is dropped on both sides before
// comparing.
func sameVersion(reported, tag string) bool {
	return strings.TrimPrefix(strings.TrimSpace(reported), "v") == strings.TrimPrefix(strings.TrimSpace(tag), "v")
}

// phase and progress tolerate a nil Reporter: the update path is also driven by
// tests and by scripts that want no drawing at all.
func phase(report Reporter, phase Phase, detail string) {
	if report != nil {
		report.Phase(phase, detail)
	}
}

func progress(report Reporter, done, total int64) {
	if report != nil {
		report.Progress(done, total)
	}
}
