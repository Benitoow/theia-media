// Package supportlog keeps Theia's structured diagnostic history on disk.
//
// The console stays concise. The rotating files keep debug records as well, so
// a problem that happened yesterday is still explainable after a restart.
package supportlog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	defaultMaxBytes = 4 << 20
	defaultBackups  = 3
)

// File is one immutable copy of a log file, ready to add to a support archive.
type File struct {
	Name string
	Data []byte
}

// Stats describes the bounded history without reading it into memory.
type Stats struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// Store is a size-bounded, rotating writer. The active file plus three backups
// caps raw diagnostic history at roughly 16 MiB per installation.
type Store struct {
	mu       sync.Mutex
	dir      string
	path     string
	maxBytes int64
	backups  int
	file     *os.File
	size     int64
}

// New builds one logger for both audiences: readable text on the console and
// JSON Lines on disk. The files always retain debug records, independently of
// the --verbose flag used for the console.
func New(dataDir string, console io.Writer, consoleLevel slog.Leveler) (*slog.Logger, *Store, error) {
	store, err := openStore(filepath.Join(dataDir, "logs"), defaultMaxBytes, defaultBackups)
	if err != nil {
		return nil, nil, err
	}
	replace := func(_ []string, attr slog.Attr) slog.Attr {
		if sensitiveKey(attr.Key) {
			return slog.String(attr.Key, "[REDACTED]")
		}
		return attr
	}
	consoleHandler := slog.NewTextHandler(console, &slog.HandlerOptions{
		Level:       consoleLevel,
		ReplaceAttr: replace,
	})
	fileHandler := slog.NewJSONHandler(store, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: replace,
	})
	return slog.New(teeHandler{handlers: []slog.Handler{consoleHandler, fileHandler}}), store, nil
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "key", "api_key", "tmdb_api_key", "token", "access_token", "refresh_token",
		"secret", "password", "authorization", "private_key", "client_config":
		return true
	default:
		return false
	}
}

func openStore(dir string, maxBytes int64, backups int) (*Store, error) {
	if maxBytes <= 0 || backups < 0 {
		return nil, fmt.Errorf("support log: invalid rotation limits")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("support log: creating directory: %w", err)
	}
	store := &Store{
		dir:      dir,
		path:     filepath.Join(dir, "theia.log"),
		maxBytes: maxBytes,
		backups:  backups,
	}
	if err := store.openActive(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) openActive() error {
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("support log: opening active file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("support log: reading active file size: %w", err)
	}
	s.file = file
	s.size = info.Size()
	return nil
}

// Write implements io.Writer for slog.JSONHandler.
func (s *Store) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return 0, os.ErrClosed
	}
	if s.size > 0 && s.size+int64(len(data)) > s.maxBytes {
		if err := s.rotateLocked(); err != nil {
			return 0, err
		}
	}
	written, err := s.file.Write(data)
	s.size += int64(written)
	return written, err
}

func (s *Store) rotateLocked() error {
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("support log: closing before rotation: %w", err)
	}
	s.file = nil
	if s.backups > 0 {
		_ = os.Remove(fmt.Sprintf("%s.%d", s.path, s.backups))
		for index := s.backups - 1; index >= 1; index-- {
			from := fmt.Sprintf("%s.%d", s.path, index)
			to := fmt.Sprintf("%s.%d", s.path, index+1)
			if err := os.Rename(from, to); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("support log: rotating backup %d: %w", index, err)
			}
		}
		if err := os.Rename(s.path, s.path+".1"); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("support log: rotating active file: %w", err)
		}
	} else if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("support log: clearing active file: %w", err)
	}
	return s.openActive()
}

// Snapshot returns complete files from oldest to newest while holding rotation
// still. A support export therefore never contains half of a JSON record.
func (s *Store) Snapshot() ([]File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil, os.ErrClosed
	}
	if err := s.file.Sync(); err != nil {
		return nil, fmt.Errorf("support log: syncing active file: %w", err)
	}

	files := make([]File, 0, s.backups+1)
	for index := s.backups; index >= 1; index-- {
		path := fmt.Sprintf("%s.%d", s.path, index)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("support log: reading backup %d: %w", index, err)
		}
		files = append(files, File{Name: filepath.Base(path), Data: data})
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("support log: reading active file: %w", err)
	}
	files = append(files, File{Name: filepath.Base(s.path), Data: data})
	return files, nil
}

// Usage reports how much diagnostic history is currently retained.
func (s *Store) Usage() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	var stats Stats
	for index := 0; index <= s.backups; index++ {
		path := s.path
		if index > 0 {
			path = fmt.Sprintf("%s.%d", s.path, index)
		}
		if info, err := os.Stat(path); err == nil {
			stats.Files++
			stats.Bytes += info.Size()
		}
	}
	return stats
}

// Close flushes and closes the active file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

type teeHandler struct {
	handlers []slog.Handler
}

func (h teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h teeHandler) Handle(ctx context.Context, record slog.Record) error {
	var first error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (h teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for index, handler := range h.handlers {
		handlers[index] = handler.WithAttrs(attrs)
	}
	return teeHandler{handlers: handlers}
}

func (h teeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for index, handler := range h.handlers {
		handlers[index] = handler.WithGroup(name)
	}
	return teeHandler{handlers: handlers}
}
