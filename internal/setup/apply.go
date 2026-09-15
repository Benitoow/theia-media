package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Benitoow/theia-media/internal/config"
)

// Action is one thing the installer did, as a code rather than a sentence. The
// interface owns every word shown (decision 25), and this tool has two
// interfaces - a terminal form and flags - so neither may invent its own.
type Action struct {
	// Kind is "created-dir", "wrote-config", "installed-service",
	// "removed-service", "already-configured" or "kept-service".
	Kind string `json:"kind"`

	// Path is what the action was about, when a path is what it was about.
	Path string `json:"path,omitempty"`

	// Detail carries the platform's own mechanism - "startup-entry",
	// "scheduled-task", "systemd-user-unit", "launchd-agent" - so the interface
	// can say what kind of autostart was installed instead of guessing.
	Detail string `json:"detail,omitempty"`
}

// Result is what Apply did, in order.
type Result struct {
	Role     Role     `json:"role"`
	DataDir  string   `json:"data_dir"`
	Port     int      `json:"port"`
	Hostname string   `json:"hostname"`
	Library  []string `json:"library_paths"`
	Actions  []Action `json:"actions"`
	// Programs are the executables this installation put in place. Listed
	// separately from the actions because the shortcuts and the autostart entry
	// are about these files, and reading them out of a log of what happened
	// would be guessing.
	Programs []string `json:"programs,omitempty"`
	// ShortcutsError is why an entry could not be written, when one was asked
	// for. The programs are installed and they run: a read-only launcher folder
	// is not a reason to refuse an installation.
	ShortcutsError string `json:"shortcuts_error,omitempty"`
	// ServiceError is why an autostart entry was not installed, when one was
	// asked for. The rest of the installation still stands: a machine that
	// cannot autostart is a machine that starts by hand, which is the default
	// anyway (§11.7).
	ServiceError string `json:"service_error,omitempty"`
}

// machineFile is where the declared role is kept. It is deliberately not part of
// the server's configuration: the role is the installer's record of a decision
// about a machine, and the server's own config describes a library. A server
// does not behave differently because somebody called the machine "all-in-one",
// and it has no business reading a file about it.
const machineFile = "setup.json"

// machineRecord is what the installer remembers about this machine.
type machineRecord struct {
	Role Role `json:"role"`
	// InstalledAt is when the role was last declared. Not used for anything yet;
	// it is the one fact that makes a support conversation possible ("when did
	// you last run the installer?").
	InstalledAt time.Time `json:"installed_at"`
}

// Apply writes the plan: it creates the data directory, records the role, writes
// the server's configuration when the role serves, and installs an autostart
// entry when one was asked for.
//
// It is deliberately reversible in the ways that matter to somebody trying it:
// nothing is written outside the data directory unless a service was requested,
// and the configuration is edited through internal/config rather than by hand,
// so the settings page and this tool cannot disagree about the file's shape.
func Apply(plan Plan) (Result, error) {
	if err := plan.Validate(); err != nil {
		return Result{}, err
	}

	result := Result{
		Role:     plan.Role,
		DataDir:  plan.DataDir,
		Port:     plan.Port,
		Hostname: plan.Hostname,
		Library:  plan.LibraryPaths,
	}

	first, err := isFirstInstall(plan.DataDir)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(plan.DataDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("creating %s: %w", plan.DataDir, err)
	}
	if first {
		result.Actions = append(result.Actions, Action{Kind: "created-dir", Path: plan.DataDir})
	} else {
		result.Actions = append(result.Actions, Action{Kind: "already-configured", Path: plan.DataDir})
	}

	if err := writeMachineRecord(plan); err != nil {
		return Result{}, err
	}
	result.Actions = append(result.Actions, Action{Kind: "wrote-role", Path: filepath.Join(plan.DataDir, machineFile)})

	if plan.Role.WantsServer() {
		// Load rather than write from scratch: the settings page may already have
		// filled in a TMDB key, and an installer that reset it would be a bug
		// people would only notice when their posters disappeared.
		cfg, err := config.Load(plan.DataDir)
		if err != nil {
			return Result{}, err
		}
		cfg.Port = plan.Port
		cfg.Hostname = plan.Hostname
		// The folders are added, not replaced: an installation that already has
		// a library must not lose it because somebody ran the installer again
		// with one new folder.
		cfg.LibraryPaths = mergePaths(cfg.LibraryPaths, plan.LibraryPaths)
		if err := cfg.Save(); err != nil {
			return Result{}, fmt.Errorf("writing the configuration: %w", err)
		}
		result.Actions = append(result.Actions, Action{Kind: "wrote-config", Path: filepath.Join(plan.DataDir, "config.json")})
	}

	if plan.Service && plan.Role.OffersService() {
		// The binary to start is resolved once, here, and handed to the platform
		// mechanism: a startup entry, a systemd unit and a launchd agent must all
		// start the same thing, and none of them should be guessing.
		server, err := serverExecutable()
		if err != nil {
			result.ServiceError = err.Error()
		} else if entry, err := installAutostart(plan, server); err != nil {
			// Reported, not fatal: the installation itself succeeded, and the
			// reason is the platform's, not the user's mistake.
			result.ServiceError = err.Error()
		} else {
			result.Actions = append(result.Actions, Action{Kind: "installed-service", Detail: entry})
		}
	} else if !plan.Service {
		result.Actions = append(result.Actions, Action{Kind: "kept-service"})
	}

	return result, nil
}

func writeMachineRecord(plan Plan) error {
	record := machineRecord{Role: plan.Role, InstalledAt: time.Now().UTC()}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(plan.DataDir, machineFile), append(data, '\n'), 0o644)
}

// readMachineRecord reports the role this machine was declared to be. A machine
// that has never been through the installer has none, which is a normal answer
// and not an error: it is what a first run looks like.
func readMachineRecord(dir string) (machineRecord, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, machineFile))
	if os.IsNotExist(err) {
		return machineRecord{}, false, nil
	}
	if err != nil {
		return machineRecord{}, false, err
	}
	var record machineRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return machineRecord{}, false, fmt.Errorf("reading %s: %w", machineFile, err)
	}
	return record, true, nil
}

// isFirstInstall reports whether this data directory has ever been configured.
// It decides nothing but the wording: an installer run twice must be safe, and
// saying "created" about a directory that already existed would be a lie the
// user can see.
func isFirstInstall(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, machineFile))
	switch {
	case os.IsNotExist(err):
		return true, nil
	case err != nil:
		return false, err
	default:
		return false, nil
	}
}

// mergePaths appends the new folders to the existing ones, keeping the order and
// dropping duplicates. Case-insensitively on Windows, where two spellings of one
// folder are one folder.
func mergePaths(existing, added []string) []string {
	merged := make([]string, 0, len(existing)+len(added))
	seen := make(map[string]bool, len(existing)+len(added))
	key := func(path string) string {
		cleaned := filepath.Clean(path)
		if isWindowsPath() {
			return strings.ToLower(cleaned)
		}
		return cleaned
	}
	for _, path := range append(append([]string{}, existing...), added...) {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if seen[key(path)] {
			continue
		}
		seen[key(path)] = true
		merged = append(merged, path)
	}
	return merged
}

// Status is what the tool can say about a machine without changing it.
type Status struct {
	Role         Role              `json:"role"`
	DataDir      string            `json:"data_dir"`
	InstallDir   string            `json:"install_dir,omitempty"`
	Configured   bool              `json:"configured"`
	Port         int               `json:"port,omitempty"`
	Hostname     string            `json:"hostname,omitempty"`
	LibraryPaths []string          `json:"library_paths"`
	Artifacts    []Artifact        `json:"artifacts"`
	Autostart    Autostart         `json:"autostart"`
	Update       *UpdateStatusView `json:"update,omitempty"`
}

// Inspect reads the machine's current state and changes nothing.
func Inspect(self string) (Status, error) {
	dir, err := config.DataDir()
	if err != nil {
		return Status{}, err
	}
	// Where the programs would be. Reported even when the folder does not exist
	// yet, because "there is nothing installed" is the answer to a fair question.
	installDir, err := DefaultInstallDir()
	if err != nil {
		installDir = ""
	}
	plan := Plan{Role: DefaultRole, DataDir: dir, InstallDir: installDir}.WithDefaults(dir)

	status := Status{DataDir: dir, InstallDir: installDir, Role: DefaultRole, LibraryPaths: []string{}}
	record, declared, err := readMachineRecord(dir)
	if err != nil {
		return Status{}, err
	}
	if declared {
		status.Role = record.Role
		status.Configured = true
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
		cfg, err := config.Load(dir)
		if err != nil {
			return Status{}, err
		}
		status.Configured = true
		status.Port = cfg.Port
		status.Hostname = cfg.Hostname
		if cfg.LibraryPaths != nil {
			status.LibraryPaths = cfg.LibraryPaths
		}
	}
	status.Artifacts = plan.Artifacts(self)
	status.Autostart = autostartStatus()
	return status, nil
}

// Encode writes a result or a status as the JSON a script can read. The tool
// prints JSON rather than prose when asked for it, for the same reason the API
// sends codes: something else may be reading.
func Encode(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
