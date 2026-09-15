package setup

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	// LibraryPaths are the folders to scan. Empty is legal: the settings page
	// adds them later, and a server with no folders is a server waiting.
	LibraryPaths []string

	Port     int
	Hostname string

	// Service is whether to install an autostart entry. Off unless asked for:
	// the founding spec's §11.7 amendment keeps a manually started binary as the
	// default, and installing a service is a decision about somebody's machine.
	Service bool
}

// Plan fills in the defaults for anything the caller left empty, so a flag that
// was not passed and a field a form did not reach behave identically.
func (p Plan) WithDefaults(defaultDataDir string) Plan {
	if p.Role == "" {
		p.Role = DefaultRole
	}
	if p.DataDir == "" {
		p.DataDir = defaultDataDir
	}
	if p.Port == 0 {
		p.Port = 8383
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
	// Found is false when the file is not there. A player-only machine does not
	// need the server, and an all-in-one needs both; what is missing is reported
	// rather than fetched, because a download is the updater's business and the
	// digest discipline lives with it.
	Found bool
}

// Artifacts lists what the chosen role needs and whether it is present, looking
// beside the installer first and then on PATH. It changes nothing.
func (p Plan) Artifacts(self string) []Artifact {
	wanted := []struct {
		name    string
		needed  bool
		purpose string
	}{
		{"theia-server", p.Role.WantsServer(), "serves the library"},
		{"theia-player", p.Role.WantsPlayer(), "watches films"},
	}
	found := make([]Artifact, 0, len(wanted))
	for _, want := range wanted {
		if !want.needed {
			continue
		}
		exe := want.name
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		artifact := Artifact{Name: exe}
		if beside := filepath.Join(filepath.Dir(self), exe); fileExists(beside) {
			artifact.Path = beside
			artifact.Found = true
		} else if onPath, err := exec.LookPath(exe); err == nil {
			artifact.Path = onPath
			artifact.Found = true
		}
		found = append(found, artifact)
	}
	return found
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
