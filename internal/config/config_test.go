package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCreatesDefaultsOnFirstLaunch(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}

	if cfg.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.Hostname != DefaultHostname {
		t.Errorf("Hostname = %q, want %q", cfg.Hostname, DefaultHostname)
	}
	if cfg.LibraryPaths == nil {
		t.Error("LibraryPaths is nil; it should be an empty slice so it marshals as [] rather than null")
	}

	// First launch must leave a file behind, otherwise there is nothing for the
	// user to edit and nothing for the settings page to write to.
	if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
		t.Errorf("no config file written on first launch: %v", err)
	}
}

func TestLoadFillsGapsInAPartialFile(t *testing.T) {
	dir := t.TempDir()
	// A hand-edited file that sets one key and omits the rest still has to
	// produce a working server.
	write(t, dir, `{"port": 9000}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want the value from the file (9000)", cfg.Port)
	}
	if cfg.Hostname != DefaultHostname {
		t.Errorf("Hostname = %q, want the default %q", cfg.Hostname, DefaultHostname)
	}
}

func TestLoadAcceptsAnEmptyObject(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}
	if cfg.Port != DefaultPort || cfg.Hostname != DefaultHostname {
		t.Errorf("got port=%d hostname=%q, want the defaults", cfg.Port, cfg.Hostname)
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"port": `)

	// Silently falling back to defaults would discard the user's real settings
	// on a stray keystroke; refusing to start is the safer failure.
	if _, err := Load(dir); err == nil {
		t.Fatal("Load accepted a truncated config file, want an error")
	}
}

func TestSaveRoundTrips(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}

	cfg.Port = 9123
	cfg.LibraryPaths = []string{filepath.Join("some", "films")}
	cfg.TMDBAPIKey = "override"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save returned an unexpected error: %v", err)
	}

	reloaded, err := Load(dir)
	if err != nil {
		t.Fatalf("reloading returned an unexpected error: %v", err)
	}
	if reloaded.Port != 9123 {
		t.Errorf("Port = %d, want 9123", reloaded.Port)
	}
	if reloaded.TMDBAPIKey != "override" {
		t.Errorf("TMDBAPIKey = %q, want %q", reloaded.TMDBAPIKey, "override")
	}
	if len(reloaded.LibraryPaths) != 1 {
		t.Errorf("LibraryPaths = %v, want one entry", reloaded.LibraryPaths)
	}
}

func TestSaveLeavesNoTemporaryFilesBehind(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}
	for range 3 {
		if err := cfg.Save(); err != nil {
			t.Fatalf("Save returned an unexpected error: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("data directory holds %v, want only %s", names, fileName)
	}
}

func TestSaveWritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir); err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	var into map[string]any
	if err := json.Unmarshal(data, &into); err != nil {
		t.Fatalf("the written config is not valid JSON: %v", err)
	}
	if _, ok := into["dir"]; ok {
		t.Error("the unexported data directory leaked into the serialized config")
	}
}

func TestDataDirHonoursTheEnvironmentOverride(t *testing.T) {
	t.Setenv("THEIA_DATA_DIR", filepath.Join("custom", "place"))

	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir returned an unexpected error: %v", err)
	}
	if want := filepath.Join("custom", "place"); got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestLocalOverridesReplaceLoadedValues(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"port": 9000, "hostname": "media"}`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}
	local := filepath.Join(t.TempDir(), localFileName)
	if err := os.WriteFile(local, []byte(`{"port": 7777, "tmdb_api_key": "dev-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.applyLocalOverrides(local); err != nil {
		t.Fatalf("applyLocalOverrides returned an unexpected error: %v", err)
	}

	if cfg.Port != 7777 {
		t.Errorf("Port = %d, want the override 7777", cfg.Port)
	}
	if cfg.TMDBAPIKey != "dev-key" {
		t.Errorf("TMDBAPIKey = %q, want the override", cfg.TMDBAPIKey)
	}
	// Untouched by the overlay, so the loaded value has to survive.
	if cfg.Hostname != "media" {
		t.Errorf("Hostname = %q, want the loaded value %q", cfg.Hostname, "media")
	}
	if !cfg.HasLocalOverrides() {
		t.Error("HasLocalOverrides() = false after applying an overlay")
	}
}

func TestLocalOverridesAreNeverPersisted(t *testing.T) {
	// The whole point: a development key in config.local.json must not migrate
	// into the real configuration file, where it would outlive the file it came
	// from and survive deleting it.
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned an unexpected error: %v", err)
	}

	local := filepath.Join(t.TempDir(), localFileName)
	if err := os.WriteFile(local, []byte(`{"tmdb_api_key": "super-secret", "port": 7777}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.applyLocalOverrides(local); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save returned an unexpected error: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(written), "super-secret") {
		t.Error("the development key was written into the persisted configuration")
	}

	var got Config
	if err := json.Unmarshal(written, &got); err != nil {
		t.Fatal(err)
	}
	if got.Port != DefaultPort {
		t.Errorf("persisted port = %d, want the pre-override default %d", got.Port, DefaultPort)
	}
	// The in-memory value still reflects the override; only the file is clean.
	if cfg.Port != 7777 {
		t.Errorf("in-memory port = %d, want the override to still apply at runtime", cfg.Port)
	}
}

func TestChangesSurviveSaveWhenNotOverridden(t *testing.T) {
	// Protecting overridden fields must not freeze the others: the settings
	// page still has to be able to persist a change.
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(t.TempDir(), localFileName)
	if err := os.WriteFile(local, []byte(`{"tmdb_api_key": "dev-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cfg.applyLocalOverrides(local); err != nil {
		t.Fatal(err)
	}

	cfg.Hostname = "cinema"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Hostname != "cinema" {
		t.Errorf("Hostname = %q, want the saved value %q", reloaded.Hostname, "cinema")
	}
}

func TestMissingLocalFileIsNotAnError(t *testing.T) {
	cfg := Default()
	if err := cfg.applyLocalOverrides(filepath.Join(t.TempDir(), "absent.json")); err != nil {
		t.Errorf("a missing local override should be the normal case, got: %v", err)
	}
	if cfg.HasLocalOverrides() {
		t.Error("HasLocalOverrides() = true with no local file")
	}
}

func TestMalformedLocalFileIsAnError(t *testing.T) {
	// Ignoring it silently would leave a developer wondering why their key is
	// not being picked up.
	local := filepath.Join(t.TempDir(), localFileName)
	if err := os.WriteFile(local, []byte(`{"tmdb_api_key":`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	if err := cfg.applyLocalOverrides(local); err == nil {
		t.Error("a truncated local override was accepted, want an error")
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", "(not set)"},
		{"short secrets reveal nothing", "abcd1234", "****"},
		// Deliberately not shaped like a real JWT. A fixture carrying the
		// standard HS256 header makes every "did a key leak into the repo?"
		// scan light up on a test file.
		{"long secret keeps only its ends", "NOT-A-REAL-TOKEN-just-a-fixture", "NOT-…ture"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Redact(tt.in); got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLogValueDoesNotExposeTheKey(t *testing.T) {
	cfg := Default()
	cfg.TMDBAPIKey = "a-real-looking-secret-value"

	rendered := cfg.LogValue().String()
	if strings.Contains(rendered, "a-real-looking-secret-value") {
		t.Errorf("LogValue() leaked the key: %s", rendered)
	}
	if !strings.Contains(rendered, "a-re") {
		t.Errorf("LogValue() = %s, want a redacted fingerprint of the key", rendered)
	}
}

func write(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
