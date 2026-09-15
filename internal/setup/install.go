package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Benitoow/theia-media/internal/release"
)

// Where a machine's programs live, and how they get there.
//
// Until V3.3 the installer wrote a configuration and an autostart entry and
// installed nothing: the programs stayed in whatever folder they had been
// unzipped into. That is why a launcher could not find Theia, why a shortcut had
// nothing to point at, and why deleting the download folder quietly broke an
// installation. The programs are now copied into one directory, and the
// shortcuts and the autostart entry point at that directory.
//
// The release page, in exchange, only has to publish one file: the installer is
// what a person downloads, and it fetches the rest itself - verified, or not at
// all.

// DefaultInstallDir is where this user's programs go.
//
// Per-user, and never elevated: decision 120 records that this installer asks
// for no administrator rights, so the standard machine-wide `Program Files` is
// out of reach by design. On Windows that leaves
// `%LOCALAPPDATA%\Programs\Theia`, which is what per-user installers use and
// what a launcher indexes.
//
// The Unix path is written down for the same reason: it is what the code does.
// It is **unverified** - V3.3 has only ever been run on Windows - and saying so
// is better than implying a promise nobody has tested.
func DefaultInstallDir() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("setup: LOCALAPPDATA is not set, so there is nowhere to install to")
		}
		return filepath.Join(base, "Programs", "Theia"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("setup: finding the home directory: %w", err)
	}
	return filepath.Join(home, ".local", "lib", "theia"), nil
}

// Phase is what an installation is doing, as a code rather than a sentence:
// decision 25 says the interface owns every word somebody reads, and this tool
// has two interfaces.
type Phase string

const (
	PhaseChecking    Phase = "checking"    // asking the release page what exists
	PhaseDownloading Phase = "downloading" // fetching one program
	PhaseExtracting  Phase = "extracting"  // unpacking a bundle
	PhaseInstalling  Phase = "installing"  // copying into the installation
	PhaseDone        Phase = "done"
)

// Reporter is told what the installation is doing while it does it. A terminal
// draws a bar from it, a script ignores it, a test passes nil.
//
// Detail is a code - "server", "player", a path - never a sentence.
type Reporter interface {
	Phase(phase Phase, detail string)
	Progress(done, total int64)
}

// Source says where a program that is not installed yet comes from.
type Source struct {
	// From is a folder or a .zip somebody already has. Empty means: look beside
	// the installer, then ask the release page.
	From string

	// Force installs the programs again even when the installation already holds
	// them. It exists because an installation has no other way to refresh the
	// player: the server updates itself through the updater and the player has
	// no such path, so re-running the installer is what a person does - and it
	// used to do nothing at all.
	Force bool

	// APIBase points at a mirror instead of GitHub. Tests point it at a stub,
	// which is the only way to exercise a download without publishing a release.
	APIBase string
	Repo    string
	Client  *http.Client
}

// program is one executable this installation needs, and what has to travel
// with it.
type program struct {
	// base is the name inside the installation, without its extension.
	base string
	// asset is the name the release publishes it under.
	asset string
	// bundle is true when the asset is a zip rather than a bare executable.
	bundle bool
	// files are the members a bundle must contain. It is the same list the
	// release pipeline checks before publishing, because a player bundle missing
	// its licence is a licence breach rather than an incomplete download.
	files []string
}

// Label is the code the interface turns into a word: "server", "player".
func (p program) Label() string { return strings.TrimPrefix(p.base, "theia-") }

// programsFor lists what a role needs on a platform, in the order a person would
// install them: the server first, because it is the thing that serves.
func programsFor(role Role, goos, goarch string) []program {
	var wanted []program
	if role.WantsServer() {
		wanted = append(wanted, program{
			base:  "theia-server",
			asset: release.ServerName(goos, goarch),
		})
	}
	if role.WantsPlayer() {
		entry := program{
			base:   "theia-player",
			asset:  release.PlayerName(goos, goarch),
			bundle: true,
			files:  bundleFiles(goos),
		}
		wanted = append(wanted, entry)
	}
	return wanted
}

// bundleFiles is what must sit beside the player. On Windows that is the engine
// and its two texts; only Windows is pinned (player/libmpv.json carries one
// platform, and an entry for a platform nobody has run would be a claim rather
// than a pin), so elsewhere the list is the executable alone and the honest
// answer is that the engine's name is not pinned yet.
func bundleFiles(goos string) []string {
	executable := "theia-player"
	if goos == "windows" {
		executable += ".exe"
		return []string{executable, "libmpv-2.dll", "LICENSE-libmpv.txt", "NOTICE.md"}
	}
	return []string{executable}
}

// InstallPrograms puts every program the role needs into the installation
// directory, fetching what is missing, and reports what it did.
//
// A program is either there or it is not: each one is completed and verified, or
// nothing of it is left behind. The first failure stops the installation,
// because a machine with half a product is harder to explain than a machine
// where nothing happened yet.
func InstallPrograms(ctx context.Context, plan Plan, source Source, report Reporter) ([]Action, error) {
	if strings.TrimSpace(plan.InstallDir) == "" {
		dir, err := DefaultInstallDir()
		if err != nil {
			return nil, err
		}
		plan.InstallDir = dir
	}
	// Whether this call created the folder decides whether a failure may remove
	// it again: an installation that could not fetch anything must not leave an
	// empty directory where the next person looks for a product.
	created := !fileExists(plan.InstallDir)
	if err := os.MkdirAll(plan.InstallDir, 0o755); err != nil {
		return nil, fmt.Errorf("setup: creating %s: %w", plan.InstallDir, err)
	}
	abandon := func(actions []Action, err error) ([]Action, error) {
		if created {
			// Only ever succeeds when the folder is empty, which is the case
			// this exists for: a download that failed before writing anything.
			os.Remove(plan.InstallDir)
		}
		return actions, err
	}

	if report != nil {
		report.Phase(PhaseChecking, plan.InstallDir)
	}
	// The release page is asked once, and only if something is actually
	// missing: an installation made from a downloaded archive must work with no
	// network at all, which is what makes the archive worth publishing.
	var latest *release.Release

	actions := make([]Action, 0, 2)
	for _, want := range programsFor(plan.Role, runtime.GOOS, runtime.GOARCH) {
		found, origin, err := findProgram(plan, source, want)
		if err != nil {
			return abandon(actions, err)
		}
		if found {
			actions = append(actions, Action{
				Kind:   "installed-program",
				Path:   filepath.Join(plan.InstallDir, executableName(want)),
				Detail: origin,
			})
			continue
		}

		if latest == nil {
			if report != nil {
				report.Phase(PhaseChecking, want.Label())
			}
			rel, err := release.Latest(ctx, source.Client, source.APIBase, source.Repo)
			if err != nil {
				return abandon(actions, &InstallError{
					Reason: ReasonReleaseUnavailable,
					Detail: want.Label(),
					Err:    err,
				})
			}
			latest = &rel
		}

		asset, err := latest.Named(want.asset)
		if err != nil {
			return abandon(actions, &InstallError{
				Reason: ReasonNotPublished,
				Detail: want.Label(),
				Err:    err,
			})
		}
		if err := fetchProgram(ctx, plan, source, want, asset, latest.Tag, report); err != nil {
			return abandon(actions, err)
		}
		actions = append(actions, Action{
			Kind:   "downloaded-program",
			Path:   filepath.Join(plan.InstallDir, executableName(want)),
			Detail: latest.Tag,
		})
	}
	if report != nil {
		report.Phase(PhaseDone, plan.InstallDir)
	}
	return actions, nil
}

func executableName(want program) string {
	if runtime.GOOS == "windows" {
		return want.base + ".exe"
	}
	return want.base
}

// findProgram reports whether the program is already in place, and if not, puts
// it there from a local source when there is one.
//
// The order is deliberate: the installation directory first (a second run of the
// installer must not download a hundred megabytes again), then what the person
// already has, then the network. --force skips the first step, which is the only
// way to refresh a program that is already installed - the player has no updater
// of its own.
func findProgram(plan Plan, source Source, want program) (bool, string, error) {
	installed := filepath.Join(plan.InstallDir, executableName(want))
	if fileExists(installed) && !source.Force {
		return true, "already-installed", nil
	}

	if source.From != "" {
		if err := placeFrom(source.From, plan, want); err != nil {
			return false, "", err
		}
		return true, originOf(source.From), nil
	}

	// Beside the installer: an unpacked release, which is what a person has
	// after extracting the archive by hand.
	self, err := os.Executable()
	if err == nil {
		beside := filepath.Dir(self)
		if hasProgram(beside, want) {
			if err := placeFrom(beside, plan, want); err != nil {
				return false, "", err
			}
			return true, "beside-installer", nil
		}
	}
	return false, "", nil
}

func originOf(from string) string {
	if strings.EqualFold(filepath.Ext(from), ".zip") {
		return "archive"
	}
	return "folder"
}

// hasProgram reports whether a folder holds what this program needs - the
// executable for the server, the whole bundle for the player.
func hasProgram(dir string, want program) bool {
	for _, name := range acceptedNames(want) {
		if !fileExists(filepath.Join(dir, name)) {
			continue
		}
		if !want.bundle {
			return true
		}
		// A player without its engine looks installed and does not start, so the
		// companions are required here rather than discovered at playback time.
		complete := true
		for _, file := range want.files {
			if file == executableName(want) {
				continue
			}
			if !fileExists(filepath.Join(dir, file)) {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}

// acceptedNames is every name the executable may have on disk, most specific
// first: the short name a working tree produces, and the published one.
func acceptedNames(want program) []string {
	return artifactNames(want.base)
}

// placeFrom copies a program out of a folder or an archive into the
// installation directory.
//
// The files are written beside their final names and renamed into place, so an
// interrupted copy cannot leave half an executable where the autostart entry
// will look for one.
func placeFrom(from string, plan Plan, want program) error {
	info, err := os.Stat(from)
	if err != nil {
		return &InstallError{
			Reason: ReasonMissingFromSource,
			Detail: from,
			Err:    fmt.Errorf("setup: %s cannot be read: %w", from, err),
		}
	}
	if info.IsDir() {
		return placeFromFolder(from, plan, want)
	}
	if strings.EqualFold(filepath.Ext(from), ".zip") {
		return placeFromArchive(from, plan, want)
	}
	return &InstallError{
		Reason: ReasonMissingFromSource,
		Detail: from,
		Err:    fmt.Errorf("setup: %s is neither a folder nor a .zip", from),
	}
}

func placeFromFolder(dir string, plan Plan, want program) error {
	source := ""
	for _, name := range acceptedNames(want) {
		if fileExists(filepath.Join(dir, name)) {
			source = filepath.Join(dir, name)
			break
		}
	}
	if source == "" {
		return &InstallError{
			Reason: ReasonMissingFromSource,
			Detail: want.Label(),
			Err: fmt.Errorf("setup: %s is not in %s (looked for %s)",
				want.Label(), dir, strings.Join(acceptedNames(want), ", ")),
		}
	}
	if err := copyInto(source, plan.InstallDir, executableName(want)); err != nil {
		return err
	}
	if !want.bundle {
		return nil
	}
	for _, file := range want.files {
		if file == executableName(want) {
			continue
		}
		from := filepath.Join(dir, file)
		if !fileExists(from) {
			return &InstallError{
				Reason: ReasonIncompleteBundle,
				Detail: file,
				Err:    fmt.Errorf("setup: %s is missing from %s, and the player does not start without it", file, dir),
			}
		}
		if err := copyInto(from, plan.InstallDir, file); err != nil {
			return err
		}
	}
	return nil
}

// placeFromArchive takes a program out of a zip.
//
// A bundle is extracted whole, licence included, and every member is required.
// A bare executable is a different problem: it may be named either way inside
// the archive, and it lands under the short name whatever it was called there -
// so the archive is searched for one accepted name at a time, and only the one
// that exists is asked for.
func placeFromArchive(archive string, plan Plan, want program) error {
	if want.bundle {
		if _, err := release.Extract(archive, plan.InstallDir, want.files); err != nil {
			return &InstallError{Reason: ReasonIncompleteBundle, Detail: want.Label(), Err: err}
		}
		return nil
	}

	staging, err := os.MkdirTemp("", "theia-archive-")
	if err != nil {
		return fmt.Errorf("setup: creating a staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	var last error
	for _, name := range acceptedNames(want) {
		if _, err := release.Extract(archive, staging, []string{name}); err != nil {
			last = err
			continue
		}
		return copyInto(filepath.Join(staging, name), plan.InstallDir, executableName(want))
	}
	return &InstallError{
		Reason: ReasonMissingFromSource,
		Detail: want.Label(),
		Err: fmt.Errorf("setup: %s is not in %s (looked for %s): %w",
			want.Label(), filepath.Base(archive), strings.Join(acceptedNames(want), ", "), last),
	}
}

// fetchProgram downloads one released asset, verifies its digest, and installs
// it. The download goes to a staging directory rather than to the installation:
// a hundred megabytes that turn out to be corrupt should not appear in the
// folder the shortcuts point at, even for a moment.
func fetchProgram(ctx context.Context, plan Plan, source Source, want program, asset release.Asset, tag string, report Reporter) error {
	staging, err := os.MkdirTemp("", "theia-install-")
	if err != nil {
		return fmt.Errorf("setup: creating a staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	if report != nil {
		report.Phase(PhaseDownloading, want.Label())
	}
	progress := func(done, total int64) {
		if report != nil {
			report.Progress(done, total)
		}
	}
	path, err := asset.Download(ctx, source.Client, staging, progress)
	if err != nil {
		return &InstallError{Reason: ReasonDownloadFailed, Detail: want.Label(), Err: err}
	}

	if !want.bundle {
		if err := copyInto(path, plan.InstallDir, executableName(want)); err != nil {
			return err
		}
		return nil
	}

	if report != nil {
		report.Phase(PhaseExtracting, want.Label())
	}
	if _, err := release.Extract(path, plan.InstallDir, want.files); err != nil {
		return &InstallError{Reason: ReasonIncompleteBundle, Detail: want.Label(), Err: err}
	}
	return nil
}

// copyInto writes a file into a directory under a given name, through a
// temporary name in the same directory.
func copyInto(from, dir, name string) error {
	target := filepath.Join(dir, name)
	if same, err := filepath.Abs(from); err == nil {
		if absolute, err := filepath.Abs(target); err == nil && strings.EqualFold(same, absolute) {
			return nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("setup: creating %s: %w", dir, err)
	}
	source, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("setup: reading %s: %w", from, err)
	}
	defer source.Close()

	partial := target + ".part"
	destination, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("setup: creating %s: %w", partial, err)
	}
	_, err = io.Copy(destination, source)
	closeErr := destination.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(partial)
		return fmt.Errorf("setup: writing %s: %w", target, err)
	}
	if err := os.Rename(partial, target); err != nil {
		os.Remove(partial)
		return fmt.Errorf("setup: naming %s: %w", target, err)
	}
	return nil
}

// Install is the whole installation, in the order that matters: the programs,
// then the configuration that describes them, then the entries somebody clicks.
//
// One function serves the form and the flags, because a machine installed from a
// script and a machine installed from a form have to end up identical: that is
// the only way either can be trusted, and the only way the tests of one say
// anything about the other.
//
// The catalogue comes in because the shortcuts carry sentences - a tooltip is
// read by a person - and the language was chosen before the first byte moved.
func Install(ctx context.Context, plan Plan, source Source, text Catalogue, report Reporter) (Result, error) {
	actions, err := InstallPrograms(ctx, plan, source, report)
	result := Result{
		Role:     plan.Role,
		DataDir:  plan.DataDir,
		Port:     plan.Port,
		Hostname: plan.Hostname,
		Library:  plan.LibraryPaths,
		Actions:  actions,
	}
	for _, action := range actions {
		result.Programs = append(result.Programs, action.Path)
	}
	if err != nil {
		// What was installed stays installed, and the result says so: an
		// installer that reported nothing after putting a server in place would
		// be lying by omission.
		return result, err
	}

	applied, err := Apply(plan)
	if err != nil {
		return Result{}, err
	}
	applied.Programs = result.Programs
	applied.Actions = append(result.Actions, applied.Actions...)

	// The maintenance tool travels with the installation, before anything names
	// it: the entry Windows keeps has to point at a command that will still be
	// there in a year, and the folder somebody downloaded into is not that place.
	uninstaller := filepath.Join(plan.InstallDir, installerExecutable(runtime.GOOS))
	if err := copySelf(uninstaller); err != nil {
		applied.ShortcutsError = err.Error()
	} else {
		applied.Actions = append(applied.Actions, Action{Kind: "installed-tool", Path: uninstaller})
	}

	// The entries come next: they point at the programs, so they can only be
	// written once those are in place.
	shortcuts, failure := createShortcuts(plan, InstalledTargets(), text)
	applied.Actions = append(applied.Actions, shortcuts...)
	if failure != "" {
		applied.ShortcutsError = joinReasons(applied.ShortcutsError, failure)
	}

	// And last, the record that makes this an installed application rather than
	// a folder: without it Windows cannot list Theia, and neither can a launcher
	// that reads the same list.
	application := defaultApplication(plan, installedVersion(plan), uninstaller)
	if err := registerApplication(applicationKeyPath(registeredName), application); err != nil {
		applied.ShortcutsError = joinReasons(applied.ShortcutsError, err.Error())
	} else {
		applied.Actions = append(applied.Actions, Action{
			Kind:   "registered-application",
			Path:   applicationKeyPath(registeredName),
			Detail: application.Version,
		})
	}
	return applied, nil
}

// firstInstalled returns the first of these programs that is in a folder, which
// is how a caller finds what was actually installed without guessing at the role.
// Both published and short names are accepted, the way findArtifact does it.
func firstInstalled(dir string, bases ...string) string {
	for _, base := range bases {
		for _, name := range artifactNames(base) {
			path := filepath.Join(dir, name)
			if fileExists(path) {
				return path
			}
		}
	}
	return ""
}

// installedVersion is the version recorded in the applications list.
//
// It is asked of the installed server rather than taken from this tool, because
// the two are not always the same number: an older installer fetches the current
// release, and a list that said 3.3.0 about a 3.4.0 server would be wrong in the
// one place somebody looks to find out what they have. Only the server is asked -
// the player has no way to report a version without opening its window.
func installedVersion(plan Plan) string {
	if path := firstInstalled(plan.InstallDir, "theia-server"); path != "" {
		if reported, err := binaryVersion(path); err == nil && reported != "" {
			return reported
		}
	}
	if plan.Version != "" {
		return plan.Version
	}
	return "dev"
}

// joinReasons keeps every reason a non-fatal step failed: they are all worth
// printing, and the last one is not more important than the first.
func joinReasons(existing, added string) string {
	if existing == "" {
		return added
	}
	return existing + "; " + added
}

// InstallError is a failure with a reason code, so the interface can say what
// happened in the user's own language instead of printing a Go error.
type InstallError struct {
	Reason string
	Detail string
	Err    error
}

// The reasons, as codes. They are the contract with the catalogues.
const (
	ReasonReleaseUnavailable = "release_unavailable"
	ReasonNotPublished       = "not_published"
	ReasonDownloadFailed     = "download_failed"
	ReasonIncompleteBundle   = "incomplete_bundle"
	ReasonMissingFromSource  = "missing_from_source"
)

func (e *InstallError) Error() string {
	if e.Err == nil {
		return "setup: " + e.Reason
	}
	return e.Err.Error()
}

func (e *InstallError) Unwrap() error { return e.Err }
