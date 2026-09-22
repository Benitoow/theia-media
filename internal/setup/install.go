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
// It is **unverified** - V3.3 has not been run on macOS yet - and saying so is
// better than implying a promise nobody has tested.
func DefaultInstallDir() (string, error) { return defaultInstallDir(runtime.GOOS) }

// defaultInstallDir is DefaultInstallDir with the platform passed in, because the
// platform-specific answers below have to be reachable by a test: this suite runs
// on one machine and has to be able to ask what another one would do.
func defaultInstallDir(goos string) (string, error) {
	if goos == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("setup: LOCALAPPDATA is not set, so there is nowhere to install to")
		}
		return filepath.Join(base, "Programs", "Theia"), nil
	}
	home, err := homeDirectory()
	if err != nil {
		return "", fmt.Errorf("setup: finding the home directory: %w", err)
	}
	return filepath.Join(home, ".local", "lib", "theia"), nil
}

// homeDirectory is this user's home directory, through one function rather than
// os.UserHomeDir at every call site: a test points USERPROFILE at a throwaway
// directory - the way the Windows tests point APPDATA at one - and every path
// below follows it.
func homeDirectory() (string, error) { return os.UserHomeDir() }

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
	// them. It explains itself as "the player had no other way to be refreshed",
	// and since decision 143 the player does: `--update-player`. What remains is
	// what it always claimed to be - a reinstall - for an installation that is
	// wrong on disk: a build from a working tree, a bundle somebody unpacked by
	// hand, files that disagree with each other.
	Force bool

	// APIBase points at a mirror instead of GitHub. Tests point it at a stub,
	// which is the only way to exercise a download without publishing a release.
	APIBase string
	Repo    string
	Client  *http.Client
}

// program is one executable this installation needs, and what has to travel
// with it.
//
// A program now carries two names that have nothing to do with each other, and
// keeping them apart is the point of three fields rather than one:
//
//   - label is the code a *sentence* calls it by ("server", "player", "theia"),
//     and the interface looks it up in the catalogue;
//   - base is the program's own name, which is what the accepted names on disk
//     are derived from;
//   - installed is the path it actually has inside the installation, which on
//     macOS is an executable buried inside an application bundle.
//
// Overloading base with that last one is what this shape exists to avoid: a
// macOS player is not called `Theia.app/Contents/MacOS/theia-player`, and a
// sentence about it does not say "Theia.app" either.
type program struct {
	// label is the code the interface turns into a word: "server", "player",
	// "theia". Every one of them is a key in the catalogue ("program.server").
	label string

	// base is the program's own name, without its extension and without the
	// directory a bundle keeps it in.
	base string

	// installed is where the program's executable is, relative to the
	// installation directory, with forward slashes: a file name on Windows and
	// Linux, and the executable inside the application bundle on macOS.
	installed string

	// tree is the directory an application bundle installs as, when the bundle is
	// a tree rather than a set of files: "Theia.app" on macOS, empty on Windows,
	// where the player's four files sit directly in the installation.
	tree string

	// asset is the name the release publishes it under.
	asset string
	// bundle is true when the asset is a zip rather than a bare executable.
	bundle bool
	// files are the members a bundle must contain, as paths relative to the
	// installation directory. It is the same list the release pipeline checks
	// before publishing, because a player bundle missing its licence is a licence
	// breach rather than an incomplete download.
	files []string
	// names is what this program may be called on disk, most specific first, when
	// the pair diskNames builds is not the whole answer - which is the macOS
	// player, whose bundle and whose executable inside it are both real names.
	names []string
}

// Label is the code the interface turns into a word: "server", "player".
func (p program) Label() string { return p.label }

// launcherBase is the name of the `theia` command inside the installation. The
// program's own name is `theia`; the release publishes it under a name that says
// what it is, because `theia-windows-amd64.exe` was the server's retired alias
// (decision 119) and a folder still holding that download must never be read as
// a launcher.
const launcherBase = "theia"

// The shape of the macOS player, which is an application bundle rather than the
// four files the Windows player is: the published asset is a zip whose members
// are this tree, and the installation is that tree. A person opens the directory
// in Finder and the system runs the executable inside it, so the paths below are
// what the bundle is *made of*, not an arrangement this installer chooses.
const (
	darwinPlayerTree       = "Theia.app"
	darwinPlayerExecutable = darwinPlayerTree + "/Contents/MacOS/theia-player"
	darwinPlayerEngine     = darwinPlayerTree + "/Contents/Frameworks/libmpv.dylib"
	darwinPlayerLicence    = darwinPlayerTree + "/Contents/Resources/LICENSE-libmpv.txt"
	darwinPlayerNotice     = darwinPlayerTree + "/Contents/Resources/NOTICE.md"
)

// programsFor lists what a role needs on a platform, in the order a person would
// install them: the server first, because it is the thing that serves.
func programsFor(role Role, goos, goarch string) []program {
	var wanted []program
	if role.WantsServer() {
		wanted = append(wanted, program{
			label:     "server",
			base:      "theia-server",
			installed: executablePath("theia-server", goos),
			names:     diskNames("theia-server", goos, goarch),
			asset:     release.ServerName(goos, goarch),
		})
	}
	if role.WantsPlayer() {
		wanted = append(wanted, playerProgram(goos, goarch))
	}
	if publishesLauncher(goos) {
		// The launcher goes last - the server and the player are what the machine
		// is for, and a launcher that failed to arrive should not be why neither
		// is installed.
		//
		// Its names are spelled out rather than derived, because its published
		// asset is deliberately not `theia-<os>-<arch>` (decision 119 retired that
		// name, and it belonged to the server): what a folder beside the installer
		// can hold is the short `theia` and `theia-launcher-<os>-<arch>`.
		wanted = append(wanted, program{
			label:     "theia",
			base:      launcherBase,
			installed: executablePath(launcherBase, goos),
			names:     []string{executablePath(launcherBase, goos), release.LauncherName(goos, goarch)},
			asset:     release.LauncherName(goos, goarch),
		})
	}
	return wanted
}

// publishesLauncher reports whether the release publishes the `theia` command
// for a platform. Windows and macOS have one; the other targets have neither a
// player nor a launcher, which is a fact about the release surface rather than
// about the platforms (internal/release owns every published name).
func publishesLauncher(goos string) bool { return goos == "windows" || goos == "darwin" }

// playerProgram describes the native player, which is a different shape on macOS.
func playerProgram(goos, goarch string) program {
	player := program{
		label:  "player",
		base:   "theia-player",
		asset:  release.PlayerName(goos, goarch),
		bundle: true,
		files:  bundleFiles(goos),
	}
	if goos == "darwin" {
		player.installed = darwinPlayerExecutable
		player.tree = darwinPlayerTree
		// Both names are real: the bundle is what a person drags to their
		// Applications folder and what the installer links, and the executable
		// inside it is the file an installation made by hand - or `--check` -
		// has to be able to find.
		player.names = []string{darwinPlayerTree, darwinPlayerExecutable}
		return player
	}
	player.installed = executablePath("theia-player", goos)
	player.names = diskNames("theia-player", goos, goarch)
	return player
}

// bundleFiles is what a player bundle must carry, as paths relative to the
// installation: the four files the Windows player is, or the macOS application
// tree and the members inside it that make it start and make it legal.
//
// The macOS list begins with the tree itself because the release publishes a zip
// whose members are that tree, and an extraction is asked for exactly these
// names: the tree brings everything with it, and each member after it is a
// second question whose answer decides whether a bundle missing its engine or
// its licence is refused. Windows is pinned and verified; the macOS list is what
// the player's bundle is *supposed* to contain, and the Mac has to confirm it
// (the tree is written down here and in the release workflow, and the two must
// agree).
func bundleFiles(goos string) []string {
	switch goos {
	case "windows":
		return []string{"theia-player.exe", "libmpv-2.dll", "LICENSE-libmpv.txt", "NOTICE.md"}
	case "darwin":
		return []string{
			darwinPlayerTree,
			darwinPlayerExecutable,
			darwinPlayerEngine,
			darwinPlayerLicence,
			darwinPlayerNotice,
		}
	}
	// Everywhere else the player is the executable alone: no engine's name is
	// pinned for a platform nobody has run, and an entry that claimed one would be
	// a pin that means nothing.
	return []string{"theia-player"}
}

// executablePath is where a program that is a single file lives inside the
// installation: its own name, and the extension this platform gives it.
func executablePath(base, goos string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

// diskNames is every name a bare program may have on disk, most specific first:
// the short name a working tree produces, and the platform-qualified one the
// release publishes.
//
// The platform is a parameter rather than runtime.GOOS because programsFor now
// answers for a named platform - which is what lets the suite that runs on
// Windows pin what macOS gets.
func diskNames(base, goos, goarch string) []string {
	extension := ""
	if goos == "windows" {
		extension = ".exe"
	}
	return []string{
		base + extension,
		fmt.Sprintf("%s-%s-%s%s", base, goos, goarch, extension),
	}
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
				Path:   installedPath(plan.InstallDir, want),
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
			Path:   installedPath(plan.InstallDir, want),
			Detail: latest.Tag,
		})
	}
	if report != nil {
		report.Phase(PhaseDone, plan.InstallDir)
	}
	return actions, nil
}

// executableName is the path a program's executable has inside the installation,
// relative to the installation directory, with forward slashes on every platform.
//
// It is carried by the program rather than derived here, because the two shapes
// disagree - a Windows player is `theia-player.exe`, a macOS one is
// `Theia.app/Contents/MacOS/theia-player` - and a test that runs on Windows has
// to be able to ask about macOS.
func executableName(want program) string {
	if want.installed != "" {
		return want.installed
	}
	return want.base
}

// installedPath is where a program's executable is, under an installation
// directory.
func installedPath(dir string, want program) string {
	return filepath.Join(dir, filepath.FromSlash(executableName(want)))
}

// programExecutable is the file name a program has inside the installation,
// for the callers that name one without describing what it needs.
//
// It reads this machine's platform because those callers are the Windows
// registry entries, which exist nowhere else.
func programExecutable(base string) string { return executablePath(base, runtime.GOOS) }

// findProgram reports whether the program is already in place, and if not, puts
// it there from a local source when there is one.
//
// The order is deliberate: the installation directory first (a second run of the
// installer must not download a hundred megabytes again), then what the person
// already has, then the network. --force skips the first step, which is the only
// way to refresh a program that is already installed - the player has no updater
// of its own.
func findProgram(plan Plan, source Source, want program) (bool, string, error) {
	installed := installedPath(plan.InstallDir, want)
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
		path := filepath.Join(dir, filepath.FromSlash(name))
		if !want.bundle {
			if fileExists(path) {
				return true
			}
			continue
		}
		// A bundle's first accepted name is its tree, which is a directory where
		// the other platforms' bundles are a list of files, so "is it there" is
		// the question - and a directory standing under a program's name is not
		// the program, which is why the flat case still asks for a file.
		if !pathExists(path) {
			continue
		}
		// A player without its engine looks installed and does not start, so the
		// members are required here rather than discovered at playback time.
		if missingBundleMember(dir, want) == "" {
			return true
		}
	}
	return false
}

// missingBundleMember names the first member a bundle needs that is not in place,
// and "" when the bundle is complete.
//
// Two members are not asked for by their installation path:
//
//   - the tree root is a directory rather than a file on macOS, and the only
//     place where a program is not a file;
//   - the flat executable of a Windows bundle is skipped entirely, because a
//     folder beside the installer holds the name the *release* published
//     (`theia-player-windows-amd64.exe`) and asking for the short name there
//     would call a complete bundle incomplete.
func missingBundleMember(dir string, want program) string {
	for _, member := range want.files {
		if want.tree == "" && member == want.installed {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(member))
		if member == want.tree {
			if !isDirectory(path) {
				return member
			}
			continue
		}
		if !fileExists(path) {
			return member
		}
	}
	return ""
}

// requireBundle reports the first member of a bundle that did not arrive, and
// nothing when every one of them did. The question and the answer are the same
// on every platform: a player without its engine does not start, and one without
// its licence is a licence breach rather than an installation.
func requireBundle(dir string, want program) error {
	member := missingBundleMember(dir, want)
	if member == "" {
		return nil
	}
	return &InstallError{
		Reason: ReasonIncompleteBundle,
		Detail: member,
		Err:    fmt.Errorf("setup: %s is missing from %s, and the player does not start without it", member, dir),
	}
}

// acceptedNames is every name the executable may have on disk, most specific
// first: the short name a working tree produces, and the published one.
func acceptedNames(want program) []string {
	if len(want.names) > 0 {
		return want.names
	}
	return artifactNames(want.base)
}

// isDirectory reports whether a path is a directory, which is what a macOS
// application bundle is: the one place where a program is not a file.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// pathExists reports whether anything at all is at a path, file or directory.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	// A bundle that is an application tree is copied whole, because the
	// executable inside it is useless without the Frameworks and Resources
	// siblings the system expects to find beside it. This is also the only branch
	// where a source folder and the installation hold exactly the same shape, so
	// the checks below can be the ones an installation is checked with.
	if want.tree != "" {
		tree := filepath.Join(dir, filepath.FromSlash(want.tree))
		if isDirectory(tree) {
			if err := copyTree(tree, filepath.Join(plan.InstallDir, filepath.FromSlash(want.tree))); err != nil {
				return err
			}
			return requireBundle(plan.InstallDir, want)
		}
	}

	source := ""
	for _, name := range acceptedNames(want) {
		if fileExists(filepath.Join(dir, filepath.FromSlash(name))) {
			source = filepath.Join(dir, filepath.FromSlash(name))
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
		from := filepath.Join(dir, filepath.FromSlash(file))
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

// copyTree copies a directory into the installation, keeping its shape.
//
// The macOS player is an application bundle - a directory the system insists on -
// so its four members are not four files beside each other but a tree whose
// relative layout is part of the program. Copying it whole also means a
// half-copied bundle cannot look like an installation: what arrives is checked
// member by member afterwards, exactly as a flat bundle is.
//
// A symlink is recreated rather than followed, because a real application bundle
// carries version links inside its Frameworks - `libmpv.dylib` beside
// `libmpv.2.dylib` - and flattening one would produce a bundle that is subtly not
// the one that was published.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o755)
		case entry.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			os.Remove(target)
			return os.Symlink(link, target)
		case entry.Type().IsRegular():
			return copyInto(path, filepath.Dir(target), filepath.Base(target))
		default:
			return fmt.Errorf("setup: %s is neither a file nor a symlink, and a bundle is made of those", path)
		}
	})
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
		staged := filepath.Join(staging, filepath.FromSlash(name))
		if !fileExists(staged) {
			// A name that answered a directory entry without writing a file -
			// which an application bundle does - is not a bare executable, and
			// saying so beats copying a directory over a program.
			last = fmt.Errorf("setup: %s in %s is not a file", name, filepath.Base(archive))
			continue
		}
		return copyInto(staged, plan.InstallDir, executableName(want))
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
	entries, failure := createEntries(plan, text)
	applied.Actions = append(applied.Actions, entries...)
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

	// The names a person types, in the two places Windows looks them up: PATH
	// for a terminal, App Paths for the Run dialog and the launchers that read
	// it. Without these the programs exist and their names do not. Both are
	// Windows mechanisms, and elsewhere they are stubs - so the block is Windows'
	// alone, which is what stops an installation from reporting an App Paths
	// entry on a machine that has no such thing.
	if isWindowsPath() {
		if added, err := addToUserPath(plan.InstallDir); err != nil {
			applied.ShortcutsError = joinReasons(applied.ShortcutsError, err.Error())
		} else if added {
			applied.Actions = append(applied.Actions, Action{Kind: "added-to-path", Path: plan.InstallDir})
		}
		launcher := filepath.Join(plan.InstallDir, programExecutable(launcherBase))
		if fileExists(launcher) {
			if err := registerAppPath(programExecutable(launcherBase), launcher); err != nil {
				applied.ShortcutsError = joinReasons(applied.ShortcutsError, err.Error())
			} else {
				applied.Actions = append(applied.Actions, Action{Kind: "registered-app-path", Path: launcher})
			}
		}
	}

	// And what a Unix machine needs *said* rather than done: the installer writes
	// no shell profile, because which file a shell reads is not something a
	// program can know. When the directory holding the command is not on PATH,
	// the exact line to add is reported instead, and the interface prints it in
	// the language it is speaking (decision 25).
	if notice := pathNoticeFor(runtime.GOOS); notice != nil {
		applied.Actions = append(applied.Actions, Action{
			Kind:   "path-not-set",
			Path:   notice.Dir,
			Detail: notice.Line,
		})
	}
	return applied, nil
}

// The entries a macOS installation leaves in front of a person.
//
// A Mac has no Start Menu. What it has instead are the two things somebody
// actually uses to start a program: an application in ~/Applications, which is
// what Finder's sidebar, Spotlight and Launchpad all index, and a command in
// ~/.local/bin, which is what a terminal finds. Both are **symlinks into the
// installation** rather than copies, for exactly the reason the Windows entries
// point at the installed files instead of holding their own: one installation,
// one place to update, and nothing left behind that removing the installation
// does not reach.
//
// ~/Applications is per-user, which is the same choice decision 120 makes for the
// installation itself: no administrator rights, no machine-wide folder, nothing
// to ask anybody for.

// macEntry is one of the two: what it is called, and what it points at.
type macEntry struct {
	// Link is the symlink - what Finder or a shell sees.
	Link string
	// Target is what it points at, inside the installation.
	Target string
	// Detail is the code the interface turns into a word for the kind of entry
	// this is: "application" or "command".
	Detail string
}

// macEntries is the entries a macOS installation leaves, as paths.
//
// The application entry is only wanted by a machine that has the player: an
// entry pointing at an application that was never installed is a broken promise
// somebody double-clicks, which is the rule the Windows entries follow too.
func macEntries(role Role, home, installDir string) []macEntry {
	entries := make([]macEntry, 0, 2)
	if role.WantsPlayer() {
		entries = append(entries, macEntry{
			Link:   macApplicationLink(home),
			Target: filepath.Join(installDir, filepath.FromSlash(darwinPlayerTree)),
			Detail: "application",
		})
	}
	entries = append(entries, macEntry{
		Link:   macCommandLink(home),
		Target: filepath.Join(installDir, launcherBase),
		Detail: "command",
	})
	return entries
}

// macApplicationLink is where the application entry goes: ~/Applications, the
// per-user folder the Finder, Spotlight and Launchpad read without any
// administrator rights.
func macApplicationLink(home string) string {
	return filepath.Join(home, "Applications", darwinPlayerTree)
}

// macCommandLink is the `theia` command: ~/.local/bin, the per-user binary
// directory most shells already read, and the same place a Unix installation's
// neighbours put theirs.
func macCommandLink(home string) string {
	return filepath.Join(home, ".local", "bin", launcherBase)
}

// createEntries writes the entries an installation leaves in front of a person,
// on whichever platform this is: Start Menu and Desktop shortcuts on Windows, the
// two symlinks above on macOS.
//
// Linux gets none, for the reason decision 122 gives - a .desktop file written by
// a guess would be a claim about somebody's menu that nothing here has run - and
// because the programs are installed and start by hand on a machine this project
// has never been run on.
func createEntries(plan Plan, text Catalogue) ([]Action, string) {
	switch runtime.GOOS {
	case "windows":
		return createShortcuts(plan, InstalledTargets(), text)
	case "darwin":
		home, err := homeDirectory()
		if err != nil {
			return nil, err.Error()
		}
		return linkMacEntries(macEntries(plan.Role, home, plan.InstallDir), os.Symlink)
	}
	return nil, ""
}

// linkMacEntries creates the entries, through the function that makes a symlink.
//
// It is a parameter rather than os.Symlink called here because a Windows machine
// will not let an ordinary process create a symlink without a privilege or
// Developer Mode, and what these entries *say* - which path points at what, for
// which role - is the part worth pinning on the machine this can be tested on.
func linkMacEntries(entries []macEntry, link func(target, name string) error) ([]Action, string) {
	actions := make([]Action, 0, len(entries))
	failures := make([]string, 0, len(entries))
	for _, entry := range entries {
		// A program this machine does not have gets no entry: an entry pointing
		// at a file that is not there is a broken promise somebody double-clicks.
		if !pathExists(entry.Target) {
			failures = append(failures, fmt.Sprintf("%s: %s", entry.Link, entry.Target))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(entry.Link), 0o755); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		// An entry from an earlier installation is replaced: a symlink is the
		// installer's own kind of object and a re-install must not fail on the one
		// it wrote last time. Anything else at that path is not the installer's to
		// delete, so it is reported and left where it is.
		if existing, err := os.Lstat(entry.Link); err == nil {
			if existing.Mode()&os.ModeSymlink == 0 {
				failures = append(failures, fmt.Sprintf("%s is not a symlink, and was left alone", entry.Link))
				continue
			}
			if err := os.Remove(entry.Link); err != nil {
				failures = append(failures, err.Error())
				continue
			}
		}
		if err := link(entry.Target, entry.Link); err != nil {
			failures = append(failures, err.Error())
			continue
		}
		actions = append(actions, Action{Kind: "created-shortcut", Path: entry.Link, Detail: entry.Detail})
	}
	if len(failures) > 0 {
		return actions, strings.Join(failures, "; ")
	}
	return actions, ""
}

// removeEntries deletes the entries an installation left, on whichever platform
// this is. Windows shortcuts are removed by removeShortcuts, which needs the
// folders the shell reports; the macOS links are under the home directory and are
// removed here. Linux has none to remove.
//
// The platform is a parameter for the same reason it is everywhere else in this
// file: `--uninstall` on a Mac has to be provable on the machine the tests run
// on, and the two links are paths under a home directory that a test can point
// somewhere else.
func removeEntries(goos string) []Action {
	if goos != "darwin" {
		return nil
	}
	home, err := homeDirectory()
	if err != nil {
		return nil
	}
	return unlinkMacEntries([]string{macApplicationLink(home), macCommandLink(home)})
}

// unlinkMacEntries removes what is at each entry path.
//
// It removes exactly the paths the installer wrote, the way removeShortcuts
// removes exactly the .lnk files it wrote, and it reports only what actually
// went: a directory with something in it is not a symlink, and forcing it out
// would be deleting a person's folder rather than removing an installation.
func unlinkMacEntries(links []string) []Action {
	actions := make([]Action, 0, len(links))
	for _, link := range links {
		if err := os.Remove(link); err == nil {
			actions = append(actions, Action{Kind: "removed-shortcut", Path: link})
		}
	}
	return actions
}

// PathNotice is the one line somebody has to add to a shell profile for the
// `theia` command to be found, and the directory it is about.
//
// The line is not a sentence and is therefore not translated: it is a shell
// command, identical on every machine that has a POSIX shell. What the catalogue
// owns is the sentence that introduces it (decision 25).
type PathNotice struct {
	Dir  string `json:"dir"`
	Line string `json:"line"`
}

// commandDir is where a Unix installation puts the `theia` command: the per-user
// binary directory POSIX distributions already agree on, and which most people's
// shell already reads.
func commandDir(home string) string { return filepath.Join(home, ".local", "bin") }

// commandLine is the line to add to a shell profile, written out in full rather
// than with $HOME so that it can be pasted anywhere and can be checked by
// reading it.
func commandLine(dir string) string {
	return fmt.Sprintf(`export PATH="%s:$PATH"`, dir)
}

// pathNoticeFor is what this platform has to say about PATH for a command this
// installation may have just put in place.
//
// Windows has nothing to say: it adds the installation directory to the user's
// PATH itself, which is what the `added-to-path` action reports. A Unix machine
// is told the line to add instead, because the installer writes no shell profile
// - which file a shell reads is not something a program can know, and writing one
// would be a change nobody asked for.
//
// nil means there is nothing to say: this platform has no such convention, the
// command is not installed in that directory, or the directory is already on
// PATH.
func pathNoticeFor(goos string) *PathNotice {
	if goos == "windows" {
		return nil
	}
	home, err := homeDirectory()
	if err != nil {
		return nil
	}
	dir := commandDir(home)
	if !fileExists(filepath.Join(dir, executablePath(launcherBase, goos))) {
		return nil
	}
	if pathIsSet(dir) {
		return nil
	}
	return &PathNotice{Dir: dir, Line: commandLine(dir)}
}

// pathIsSet reports whether a directory is on this machine's PATH.
//
// It reads the environment this process was given, which is not the PATH a
// person's shell will have: an installer cannot see a login shell's environment,
// and that is exactly why the answer is a line to paste rather than a change to
// make.
func pathIsSet(dir string) bool {
	target := filepath.Clean(strings.TrimSpace(dir))
	if target == "" || target == "." {
		return false
	}
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if samePathEntry(filepath.Clean(entry), target) {
			return true
		}
	}
	return false
}

// samePathEntry compares two directory names the way this platform does: an entry
// of a PATH list is not a file name, so case matters exactly as much as the file
// system says it does. The paths are already cleaned by the caller, which is what
// makes `~/.local/bin` and `~/.local/bin/` one directory.
//
// It is deliberately not the Windows registry helper of the same shape
// (samePath): that one exists to avoid writing a duplicate entry into somebody's
// stored PATH, and this one only ever reads a list of directories.
func samePathEntry(a, b string) bool {
	if isWindowsPath() {
		return strings.EqualFold(a, b)
	}
	return a == b
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
// one place somebody looks to find out what they have. Only the server is asked.
// The player can be asked too since decision 143 - `theia-player -version` - and
// is not, because a Windows applications list wants one version and the server is
// the component that maintains its own: a list showing two numbers would be a
// worse answer to a simpler question. What the player carries is asked where it
// matters, by `--check-player`.
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
