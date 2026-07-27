// Package config loads and persists Theia's on-disk configuration.
//
// Theia is meant to run with zero configuration, so every field here has a
// working default and the file is created automatically on first launch. The
// settings page edits this same file; nothing else is expected to.
//
// There is a second, development-only layer: a config.local.json sitting in the
// working directory overrides the real configuration at runtime. That is where
// a developer's own TMDB key lives. It is listed in .gitignore, it is never
// written back by Save, and Config implements slog.LogValuer so that logging
// one cannot spill the key.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

const (
	// DefaultPort is the TCP port Theia listens on unless overridden.
	DefaultPort = 8383

	// DefaultHostname is announced over mDNS, making the server reachable at
	// "theia.local" on networks whose clients resolve mDNS hostnames.
	DefaultHostname = "theia"

	// fileName is the configuration file inside the data directory.
	fileName = "config.json"

	// localFileName is the development override, read from the working
	// directory. Never committed, never written to.
	localFileName = "config.local.json"
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

	// LibraryPaths are the directories scanned for media files.
	LibraryPaths []string `json:"library_paths"`

	// TMDBAPIKey overrides the key compiled into the binary. Empty means "use
	// the built-in key", which is the case for nearly every user -- this exists
	// for people who would rather spend their own API quota.
	TMDBAPIKey string `json:"tmdb_api_key"`

	// dir is where this config was loaded from. Not serialized.
	dir string

	// persisted holds the values as they were on disk, before any local
	// development override was layered on top, together with the set of fields
	// that override actually replaced. Save consults both so that a developer's
	// key cannot migrate from config.local.json into the real configuration.
	persisted  *Config
	overridden map[string]bool
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
// not exist yet. Fields left empty in the file fall back to their defaults, and
// a config.local.json in the working directory is layered on top afterwards.
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
	case err != nil:
		return nil, fmt.Errorf("reading %s: %w", fileName, err)
	default:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", fileName, err)
		}
		cfg.dir = dir
		cfg.applyDefaults()
	}

	if err := cfg.applyLocalOverrides(localFileName); err != nil {
		return nil, err
	}
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

// overlay mirrors Config with pointers, so that "absent from the file" is
// distinguishable from "present and set to zero".
type overlay struct {
	Port         *int      `json:"port"`
	Hostname     *string   `json:"hostname"`
	LibraryPaths *[]string `json:"library_paths"`
	TMDBAPIKey   *string   `json:"tmdb_api_key"`
}

// applyLocalOverrides layers a development config.local.json over the loaded
// configuration. A missing file is the normal case and not an error; a
// malformed one is, because silently ignoring it would leave a developer
// wondering why their key is not being used.
func (c *Config) applyLocalOverrides(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var over overlay
	if err := json.Unmarshal(data, &over); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	snapshot := *c
	snapshot.LibraryPaths = slices.Clone(c.LibraryPaths)
	snapshot.persisted = nil
	snapshot.overridden = nil

	c.persisted = &snapshot
	c.overridden = map[string]bool{}

	if over.Port != nil {
		c.Port = *over.Port
		c.overridden["port"] = true
	}
	if over.Hostname != nil {
		c.Hostname = *over.Hostname
		c.overridden["hostname"] = true
	}
	if over.LibraryPaths != nil {
		c.LibraryPaths = *over.LibraryPaths
		c.overridden["library_paths"] = true
	}
	if over.TMDBAPIKey != nil {
		c.TMDBAPIKey = *over.TMDBAPIKey
		c.overridden["tmdb_api_key"] = true
	}
	return nil
}

// Dir returns the data directory this configuration was loaded from.
func (c *Config) Dir() string { return c.dir }

// HasLocalOverrides reports whether a config.local.json was layered on top.
// Used at startup to say so out loud, since a developer running with someone
// else's port or key and not realising it is a confusing afternoon.
func (c *Config) HasLocalOverrides() bool { return len(c.overridden) > 0 }

// LogValue implements slog.LogValuer, so that logging a Config -- deliberately
// or by accident, in a crash dump or a screen-shared terminal -- cannot spill
// the TMDB key.
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("port", c.Port),
		slog.String("hostname", c.Hostname),
		slog.Int("library_paths", len(c.LibraryPaths)),
		slog.String("tmdb_api_key", Redact(c.TMDBAPIKey)),
	)
}

// KeySource says where the effective TMDB key came from. It is reported in the
// settings so that "why is it using the wrong key" is answerable without
// reading any code.
type KeySource string

const (
	KeyFromSettings  KeySource = "settings"
	KeyFromBuild     KeySource = "built-in"
	KeyFromLocalFile KeySource = "config.local.json"
	KeyMissing       KeySource = "none"
)

// ResolveTMDBKey picks the key to use, given the one compiled into this build.
//
// Order, highest first:
//
//  1. tmdb_api_key in config.json. Someone typing a key into the settings is
//     deliberately spending their own quota, and that outranks the default.
//  2. The key injected at build time. This is what a release binary ships with
//     and what makes the product work out of the box.
//  3. tmdb_api_key in config.local.json, the development fallback. It sits
//     below the built-in key on purpose: a release build behaves identically
//     wherever it runs, whatever happens to be lying around next to it.
//  4. Nothing, in which case the caller says so plainly rather than quietly
//     serving a library with no posters and no explanation.
func (c *Config) ResolveTMDBKey(builtIn string) (string, KeySource) {
	settings := c.TMDBAPIKey
	local := ""
	if c.overridden["tmdb_api_key"] {
		local = c.TMDBAPIKey
		if c.persisted != nil {
			settings = c.persisted.TMDBAPIKey
		} else {
			settings = ""
		}
	}

	switch {
	case settings != "":
		return settings, KeyFromSettings
	case builtIn != "":
		return builtIn, KeyFromBuild
	case local != "":
		return local, KeyFromLocalFile
	default:
		return "", KeyMissing
	}
}

// Redact reduces a secret to something safe to print: enough to confirm which
// credential is loaded, never enough to use it.
func Redact(secret string) string {
	const keep = 4
	switch {
	case secret == "":
		return "(not set)"
	case len(secret) <= keep*2:
		return "****"
	default:
		return secret[:keep] + "…" + secret[len(secret)-keep:]
	}
}

// Save writes the configuration back to disk. The write goes to a temporary
// file first and is then renamed over the target, so an interrupted save can
// never leave a truncated config behind.
//
// Fields that came from config.local.json are written back with their original
// on-disk values: a development override is a runtime thing, and a key that
// migrated into the real configuration file would outlive the file it came from.
func (c *Config) Save() error {
	if c.dir == "" {
		return errors.New("config: no data directory set")
	}

	out := *c
	out.LibraryPaths = slices.Clone(c.LibraryPaths)
	if c.persisted != nil {
		if c.overridden["port"] {
			out.Port = c.persisted.Port
		}
		if c.overridden["hostname"] {
			out.Hostname = c.persisted.Hostname
		}
		if c.overridden["library_paths"] {
			out.LibraryPaths = slices.Clone(c.persisted.LibraryPaths)
		}
		if c.overridden["tmdb_api_key"] {
			out.TMDBAPIKey = c.persisted.TMDBAPIKey
		}
	}

	data, err := json.MarshalIndent(out, "", "  ")
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
