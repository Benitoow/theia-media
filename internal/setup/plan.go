package setup

import (
	"fmt"
	"github.com/Benitoow/theia-media/internal/config"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Plan is what the installer is about to do, in full. It exists as a value so
// that the form, the flags and the tests all describe the same thing, and so
// that Apply can be a function of one argument rather than of the interface.
type Plan struct {
	Role Role

	// DataDir holds config.json, the database and the cache.
	DataDir string

	// InstallDir holds the programs. Empty means this user's standard place,
	// resolved by DefaultInstallDir - beside the data rather than inside it,
	// because a data directory is something people back up and move.
	InstallDir string

	// LibraryPaths are the folders to scan. Empty is legal: the settings page
	// adds them later, and a server with no folders is a server waiting.
	LibraryPaths []string

	Port     int
	Hostname string

	// Language is the language the interfaces open in and the language metadata
	// is fetched in. It is asked by the form and accepted from --lang, and it is
	// written into the server's configuration when this machine serves.
	Language string

	// Version is the release this installation came from, as the tool carries
	// it. It is a fallback: what the applications list records is the version
	// the installed server reports (see installedVersion).
	Version string

	// Service is whether to install an autostart entry. Off unless asked for:
	// the founding spec's §11.7 amendment keeps a manually started binary as the
	// default, and installing a service is a decision about somebody's machine.
	Service bool
}

// Version is the release this tool was built from.
//
// It is a variable rather than a constant because the linker injects it, the way
// it does for the server: `-X main.version` lands in main, and main hands it
// here. Anything that records a version - the applications list - falls back to
// it, and prefers the version the installed program itself reports.
var Version = "dev"

// Plan fills in the defaults for anything the caller left empty, so a flag that
// was not passed and a field a form did not reach behave identically.
func (p Plan) WithDefaults(defaultDataDir string) Plan {
	if p.Role == "" {
		p.Role = DefaultRole
	}
	if p.Version == "" {
		p.Version = Version
	}
	if p.DataDir == "" {
		p.DataDir = defaultDataDir
	}
	if p.Port == 0 {
		p.Port = 8383
	}
	if p.Language == "" {
		p.Language = config.DefaultLanguage
	}
	if p.Hostname == "" {
		p.Hostname = "theia"
	}
	if p.LibraryPaths == nil {
		p.LibraryPaths = []string{}
	}
	return p
}

// Validate reports what is wrong with a plan before anything is written.
//
// Every message names the thing that is wrong and where, because this is the
// last moment before an installation writes to somebody's disk. The interface
// turns these into sentences; the tool never prints an error code on its own.
//
// It normalises as well as checks - the data directory comes back absolute - so
// what is written is what was validated.
func (p *Plan) Validate() error {
	if !p.Role.Valid() {
		return fmt.Errorf("role %q does not describe a machine", p.Role)
	}
	if strings.TrimSpace(p.DataDir) == "" {
		return fmt.Errorf("no data directory was given")
	}
	absolute, err := filepath.Abs(p.DataDir)
	if err != nil {
		return fmt.Errorf("data directory %q cannot be resolved: %w", p.DataDir, err)
	}
	p.DataDir = absolute

	if strings.TrimSpace(p.InstallDir) != "" {
		absolute, err := filepath.Abs(p.InstallDir)
		if err != nil {
			return fmt.Errorf("installation directory %q cannot be resolved: %w", p.InstallDir, err)
		}
		p.InstallDir = absolute
	}

	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port %d is not a port", p.Port)
	}
	if p.Role.WantsServer() {
		if err := portIsFree(p.Port); err != nil {
			return err
		}
	}
	if strings.TrimSpace(p.Hostname) == "" {
		return fmt.Errorf("the mDNS hostname cannot be empty")
	}
	for _, path := range p.LibraryPaths {
		info, err := os.Stat(path)
		switch {
		case os.IsNotExist(err):
			return fmt.Errorf("the folder %s does not exist", path)
		case err != nil:
			return fmt.Errorf("the folder %s cannot be read: %w", path, err)
		case !info.IsDir():
			return fmt.Errorf("%s is a file, not a folder", path)
		}
	}
	return nil
}

// portIsFree reports whether the server could bind. A warning rather than a
// refusal would be wrong here: the server exits on a taken port, so an installer
// that accepted one would produce an installation that does not start.
//
// The check is a race by construction - the port is free when asked and may not
// be when the server starts - so it is deliberately the last, cheapest thing
// that can still be said before writing.
func portIsFree(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("port %d is already in use", port)
	}
	return listener.Close()
}

// Artifact is one executable this installation needs, and where the installer
// looked for it.
type Artifact struct {
	Name string
	Path string
	// Names lists every name accepted for this artifact, so a report can say
	// what it looked for instead of leaving somebody guessing.
	Names []string
	// Found is false when the file is not there. A player-only machine does not
	// need the server, and an all-in-one needs both; what is missing is reported
	// rather than fetched, because a download is the updater's business and the
	// digest discipline lives with it.
	Found bool
}

// Artifacts lists what the chosen role needs and whether it is present, looking
// beside the installer, then in the installation directory, then on PATH.
//
// The names it accepts are the ones a person actually has on disk, which for a
// downloaded release is `theia-server-windows-amd64.exe` and not
// `theia-server.exe`. See artifactNames.
func (p Plan) Artifacts(self string) []Artifact {
	wanted := []struct {
		name   string
		needed bool
	}{
		{"theia-server", p.Role.WantsServer()},
		{"theia-player", p.Role.WantsPlayer()},
	}
	found := make([]Artifact, 0, len(wanted))
	for _, want := range wanted {
		if !want.needed {
			continue
		}
		names := artifactNames(want.name)
		artifact := Artifact{Name: names[0], Names: names}
		if beside, err := besideInstallerRelative(self, names); err == nil {
			artifact.Path = beside
			artifact.Found = true
		} else if installed, err := inDirRelative(p.InstallDir, names); err == nil {
			artifact.Path = installed
			artifact.Found = true
		} else if onPath, err := lookPathAny(names); err == nil {
			artifact.Path = onPath
			artifact.Found = true
		}
		found = append(found, artifact)
	}
	return found
}

// inDirRelative looks for one of these names in a given directory, which is how
// an installed program is found. An empty directory finds nothing rather than
// searching the working directory by accident.
func inDirRelative(dir string, names []string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", os.ErrNotExist
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		if fileExists(path) {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

// besideInstallerRelative is besideInstaller, but against a given installer path
// so that `--check` can be pointed at one without being that file.
func besideInstallerRelative(self string, names []string) (string, error) {
	for _, name := range names {
		path := filepath.Join(filepath.Dir(self), name)
		if fileExists(path) {
			return path, nil
		}
	}
	return "", os.ErrNotExist
}

func lookPathAny(names []string) (string, error) {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
