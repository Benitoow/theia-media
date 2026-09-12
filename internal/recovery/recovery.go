// Package recovery protects the database and configuration across a binary
// update. A new executable only commits the update after its migrated database
// is open and its local health endpoint answers with the expected version.
package recovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Benitoow/theia-media/internal/db"
)

const (
	directoryName = "update-recovery"
	journalName   = "pending.json"
	databaseName  = "theia.db.snapshot"
	configName    = "config.json.snapshot"
)

type journal struct {
	FromVersion   string    `json:"from_version"`
	TargetVersion string    `json:"target_version"`
	SchemaVersion string    `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	HadConfig     bool      `json:"had_config"`
}

type Manager struct {
	dataDir  string
	execPath string
	version  string
	log      *slog.Logger
}

func New(dataDir, execPath, version string, log *slog.Logger) *Manager {
	return &Manager{dataDir: dataDir, execPath: execPath, version: version, log: log}
}

func (m *Manager) dir() string         { return filepath.Join(m.dataDir, directoryName) }
func (m *Manager) journalPath() string { return filepath.Join(m.dir(), journalName) }

// Prepare snapshots durable state and writes the journal last. A crash before
// the journal is harmless; a crash after it is recoverable.
func (m *Manager) Prepare(ctx context.Context, database *sql.DB, targetVersion string) error {
	if err := os.MkdirAll(m.dir(), 0o700); err != nil {
		return fmt.Errorf("creating update recovery directory: %w", err)
	}
	_ = os.Remove(filepath.Join(m.dir(), databaseName))
	_ = os.Remove(filepath.Join(m.dir(), configName))
	_ = os.Remove(m.journalPath())

	if err := db.Snapshot(ctx, database, filepath.Join(m.dir(), databaseName)); err != nil {
		return err
	}
	configPath := filepath.Join(m.dataDir, "config.json")
	hadConfig := false
	if _, err := os.Stat(configPath); err == nil {
		hadConfig = true
		if err := copyFile(configPath, filepath.Join(m.dir(), configName), 0o600); err != nil {
			return fmt.Errorf("snapshotting configuration: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking configuration before update: %w", err)
	}

	var schemaVersion string
	if err := database.QueryRowContext(ctx, `SELECT name FROM schema_migrations ORDER BY name DESC LIMIT 1`).Scan(&schemaVersion); err != nil {
		return fmt.Errorf("reading schema version for recovery: %w", err)
	}
	record := journal{
		FromVersion: m.version, TargetVersion: targetVersion,
		SchemaVersion: schemaVersion, CreatedAt: time.Now().UTC(), HadConfig: hadConfig,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	temporary := m.journalPath() + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("writing update recovery journal: %w", err)
	}
	if err := os.Rename(temporary, m.journalPath()); err != nil {
		return fmt.Errorf("committing update recovery journal: %w", err)
	}
	m.log.Info("prepared update recovery point", "from", m.version, "to", targetVersion)
	return nil
}

func (m *Manager) PendingFor(version string) bool {
	record, err := m.readJournal()
	return err == nil && equalVersion(record.TargetVersion, version)
}

// Abort removes a prepared point when the executable swap itself failed.
func (m *Manager) Abort() error { return os.RemoveAll(m.dir()) }

// Commit removes state kept only for rollback. The outgoing executable is
// removed separately by updater.CleanPrevious after the same health check.
func (m *Manager) Commit() error {
	if err := os.RemoveAll(m.dir()); err != nil {
		return fmt.Errorf("removing update recovery point: %w", err)
	}
	return nil
}

// Rollback restores data first and the executable last. The caller must close
// the database and listeners before invoking it.
func (m *Manager) Rollback() error {
	record, err := m.readJournal()
	if err != nil {
		return err
	}
	if !equalVersion(record.TargetVersion, m.version) {
		return fmt.Errorf("recovery point targets %s, running %s", record.TargetVersion, m.version)
	}

	databasePath := filepath.Join(m.dataDir, db.FileName)
	failedDatabase := databasePath + ".failed-update"
	_ = os.Remove(failedDatabase)
	_ = os.Remove(databasePath + "-wal")
	_ = os.Remove(databasePath + "-shm")
	if err := os.Rename(databasePath, failedDatabase); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("moving migrated database aside: %w", err)
	}
	if err := copyFile(filepath.Join(m.dir(), databaseName), databasePath+".restoring", 0o600); err != nil {
		_ = os.Rename(failedDatabase, databasePath)
		return fmt.Errorf("restoring database snapshot: %w", err)
	}
	if err := os.Rename(databasePath+".restoring", databasePath); err != nil {
		_ = os.Rename(failedDatabase, databasePath)
		return fmt.Errorf("installing database snapshot: %w", err)
	}

	configPath := filepath.Join(m.dataDir, "config.json")
	if record.HadConfig {
		if err := copyFile(filepath.Join(m.dir(), configName), configPath+".restoring", 0o600); err != nil {
			return fmt.Errorf("restoring configuration snapshot: %w", err)
		}
		if err := os.Rename(configPath+".restoring", configPath); err != nil {
			return fmt.Errorf("installing configuration snapshot: %w", err)
		}
	} else {
		_ = os.Remove(configPath)
	}

	previous := m.execPath + ".old"
	failedExecutable := m.execPath + ".failed-update"
	_ = os.Remove(failedExecutable)
	if err := os.Rename(m.execPath, failedExecutable); err != nil {
		return fmt.Errorf("moving failed executable aside: %w", err)
	}
	if err := os.Rename(previous, m.execPath); err != nil {
		_ = os.Rename(failedExecutable, m.execPath)
		return fmt.Errorf("restoring previous executable: %w", err)
	}
	if err := os.RemoveAll(m.dir()); err != nil {
		return fmt.Errorf("clearing recovery journal after rollback: %w", err)
	}
	m.log.Error("rolled back an unhealthy update", "from", record.TargetVersion, "to", record.FromVersion)
	return nil
}

func (m *Manager) readJournal() (journal, error) {
	data, err := os.ReadFile(m.journalPath())
	if err != nil {
		return journal{}, err
	}
	var record journal
	if err := json.Unmarshal(data, &record); err != nil {
		return journal{}, fmt.Errorf("reading update recovery journal: %w", err)
	}
	return record, nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func equalVersion(a, b string) bool {
	trim := func(value string) string {
		if len(value) > 0 && value[0] == 'v' {
			return value[1:]
		}
		return value
	}
	return trim(a) == trim(b)
}
