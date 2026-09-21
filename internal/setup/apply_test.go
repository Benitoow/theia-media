package setup

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/config"
)

// freePort asks the kernel for a port nobody is using, so the validation's own
// "is it free" check is not what the test measures.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestApplyWritesAConfigurationTheServerAlreadyReads(t *testing.T) {
	root := t.TempDir()
	films := filepath.Join(root, "films")
	if err := os.MkdirAll(films, 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	port := freePort(t)

	plan := Plan{
		Role:         RoleServer,
		Language:     config.LanguageFrench,
		DataDir:      dataDir,
		LibraryPaths: []string{films},
		Port:         port,
		Hostname:     "theia-test",
	}
	result, err := Apply(plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Read back through the server's own loader: an installer that writes a
	// file the product cannot read has installed nothing.
	loaded, err := config.Load(dataDir)
	if err != nil {
		t.Fatalf("the server could not read what the installer wrote: %v", err)
	}
	if loaded.Port != port {
		t.Errorf("port = %d, want %d", loaded.Port, port)
	}
	if loaded.Hostname != "theia-test" {
		t.Errorf("hostname = %q, want theia-test", loaded.Hostname)
	}
	if len(loaded.LibraryPaths) != 1 || loaded.LibraryPaths[0] != films {
		t.Errorf("library paths = %q, want %q", loaded.LibraryPaths, films)
	}
	// The language chosen at installation is the one the server hands both
	// interfaces, so it has to survive the round trip like everything else.
	if loaded.Language != config.LanguageFrench {
		t.Errorf("language = %q, want fr", loaded.Language)
	}

	// Nothing outside the data directory, and nothing that starts by itself.
	kinds := map[string]bool{}
	for _, action := range result.Actions {
		kinds[action.Kind] = true
	}
	if !kinds["created-dir"] || !kinds["wrote-config"] {
		t.Errorf("actions = %+v, want a created directory and a written configuration", result.Actions)
	}
	if kinds["installed-service"] {
		t.Error("Apply installed an autostart entry nobody asked for")
	}
	if result.ServiceError != "" {
		t.Errorf("ServiceError = %q, want empty", result.ServiceError)
	}
}

func TestRunningTheInstallerAgainAddsAFolderInsteadOfReplacingThem(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "films")
	second := filepath.Join(root, "series")
	for _, dir := range []string{first, second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := filepath.Join(root, "data")

	base := Plan{Role: RoleServer, DataDir: dataDir, Port: freePort(t), Hostname: "theia"}
	firstPlan := base
	firstPlan.LibraryPaths = []string{first}
	if _, err := Apply(firstPlan); err != nil {
		t.Fatalf("first Apply: %v", err)
	}

	// Somebody runs it again with one new folder. Losing the first library would
	// be a data-loss bug that only shows up as an empty interface.
	secondPlan := base
	secondPlan.Port = base.Port
	secondPlan.LibraryPaths = []string{second}
	result, err := Apply(secondPlan)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	loaded, err := config.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.LibraryPaths) != 2 {
		t.Fatalf("library paths = %q, want both folders", loaded.LibraryPaths)
	}
	found := false
	for _, action := range result.Actions {
		if action.Kind == "already-configured" {
			found = true
		}
	}
	if !found {
		t.Error("the second run claimed to have created the directory for the first time")
	}
}

func TestApplyKeepsAKeyItDidNotSet(t *testing.T) {
	// The settings page can have filled in a TMDB key. An installer that reset
	// it would be noticed only when the posters disappeared.
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.TMDBAPIKey = "a-key-somebody-typed"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	plan := Plan{Role: RoleAllInOne, DataDir: dataDir, Port: freePort(t), Hostname: "theia"}
	if _, err := Apply(plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	loaded, err := config.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TMDBAPIKey != "a-key-somebody-typed" {
		t.Errorf("the TMDB key was %q after installing, want it untouched", loaded.TMDBAPIKey)
	}
}

func TestInspectSaysWhetherTheMachineIsConfigured(t *testing.T) {
	root := t.TempDir()
	// THEIA_DATA_DIR is how the server itself is pointed at a throwaway
	// directory, so the inspector follows the same rule.
	t.Setenv("THEIA_DATA_DIR", filepath.Join(root, "data"))

	status, err := Inspect(filepath.Join(root, "theia-setup"))
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if status.Configured {
		t.Error("a directory with no configuration reported itself as configured")
	}

	port := freePort(t)
	plan := Plan{Role: RoleServer, DataDir: filepath.Join(root, "data"), Port: port, Hostname: "theia"}
	if _, err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	status, err = Inspect(filepath.Join(root, "theia-setup"))
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured {
		t.Fatal("a configured machine reported itself as unconfigured")
	}
	if status.Port != port {
		t.Errorf("status port = %d, want %d", status.Port, port)
	}
}

func TestArtifactsSayWhatIsMissingRatherThanFetchingIt(t *testing.T) {
	root := t.TempDir()
	// A role that watches needs a player; nothing is beside the fake installer,
	// so it must be reported missing rather than downloaded. The count is asked
	// of the product's own program list, because --check and the installation
	// have to describe the same machine.
	plan := Plan{Role: RolePlayer}
	artifacts := plan.Artifacts(filepath.Join(root, "theia-setup"))
	if want := len(programsFor(RolePlayer, runtime.GOOS, runtime.GOARCH)); len(artifacts) != want {
		t.Fatalf("a player-only machine asked for %d artifacts, want %d", len(artifacts), want)
	}
	if artifacts[0].Name != "theia-player" && artifacts[0].Name != "theia-player.exe" {
		t.Errorf("artifact = %q, want the player", artifacts[0].Name)
	}
	if artifacts[0].Found && artifacts[0].Path != "" {
		if _, err := os.Stat(artifacts[0].Path); err != nil {
			t.Errorf("the artifact was reported present at %s, which does not exist", artifacts[0].Path)
		}
	}
}

func TestThePortCheckRefusesAPortSomebodyElseHolds(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	plan := Plan{Role: RoleServer, DataDir: t.TempDir(), Port: port, Hostname: "theia"}
	err = plan.Validate()
	if err == nil {
		t.Fatalf("Validate accepted port %d, which is already bound", port)
	}
	if want := strconv.Itoa(port); !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal %q does not name the port", err)
	}
}
