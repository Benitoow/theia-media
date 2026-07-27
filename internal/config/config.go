// Package config loads and persists Theia's on-disk configuration.
//
// Theia is meant to run with zero configuration, so every field here has a
// working default and the file is created automatically on first launch. The
// settings page edits this same file; nothing else is expected to.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// DefaultPort is the TCP port Theia listens on unless overridden.
	DefaultPort = 8383

	// DefaultHostname is announced over mDNS, making the server reachable at
	// "theia.local" on networks whose clients resolve mDNS hostnames.
	DefaultHostname = "theia"

	// fileName is the configuration file inside the data directory.
	fileName = "config.json"
)

// Config is the full on-disk configuration. Every field is optional: zero
// values are replaced by defaults when the file is loaded, so a hand-edited
// file that is missing keys -- or is just "{}" -- still starts a working server.
type Config struct {
	// Port is the TCP port the HTTP server binds to.
	Port int `json:"port"`

	// Hostname is the name announced over mDNS. Clients reach the server at
	// "<hostname>.local". It has no other effect.
	Hostname string `json:"hostname"`

	// LibraryPaths are the directories scanned for media files. Empty until the
	// user adds one; the scanner lands in M1.
	LibraryPaths []string `json:"library_paths"`

	// TMDBAPIKey overrides the key compiled into the binary. Empty means "use
	// the built-in key", which is the case for nearly every user -- this exists
	// for people who would rather spend their own API quota.
	TMDBAPIKey string `json:"tmdb_api_key"`

	// dir is where this config was loaded from. Not serialized.
	dir string
}

// Default returns a configuration with every field at its default value.
func Default() Config {
	return Config{
		Port:         DefaultPort,
		Hostname:     DefaultHostname,
		LibraryPaths: []string{},
	}
}

// DataDir returns the directory holding Theia's configuration, database, cache
// and downloaded ffmpeg binary. THEIA_DATA_DIR overrides it, which is what the
// tests and the portable "run it from a USB stick" case use.
func DataDir() (string, error) {
	if v := os.Getenv("THEIA_DATA_DIR"); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating user config directory: %w", err)
	}
	// %AppData%\Theia and ~/Library/Application Support/Theia are conventionally
	// capitalized; ~/.config/theia is not.
	name := "Theia"
	if runtime.GOOS == "linux" {
		name = "theia"
	}
	return filepath.Join(base, name), nil
}

// Load reads the configuration from dir, creating it with defaults if it does
// not exist yet. Fields left empty in the file fall back to their defaults.
func Load(dir string) (*Config, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data directory %s: %w", dir, err)
	}

	cfg := Default()
	cfg.dir = dir

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// First launch. Persist the defaults so the file is there to be edited.
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		return &cfg, nil
	case err != nil:
		return nil, fmt.Errorf("reading %s: %w", fileName, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", fileName, err)
	}
	cfg.dir = dir
	cfg.applyDefaults()
	return &cfg, nil
}

// applyDefaults fills in any field the file left at its zero value.
func (c *Config) applyDefaults() {
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.Hostname == "" {
		c.Hostname = DefaultHostname
	}
	if c.LibraryPaths == nil {
		c.LibraryPaths = []string{}
	}
}

// Dir returns the data directory this configuration was loaded from.
func (c *Config) Dir() string { return c.dir }

// Save writes the configuration back to disk. The write goes to a temporary
// file first and is then renamed over the target, so an interrupted save can
// never leave a truncated config behind.
func (c *Config) Save() error {
	if c.dir == "" {
		return errors.New("config: no data directory set")
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(c.dir, fileName)
	tmp, err := os.CreateTemp(c.dir, fileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temporary config file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temporary config file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temporary config file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("replacing %s: %w", target, err)
	}
	return nil
}
