package config

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func write(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
