// Package db owns the SQLite database: opening it, migrating it, and nothing
// else. Queries live with the domain packages that need them.
//
// The driver is modernc.org/sqlite, which is pure Go. This is not a preference
// -- mattn/go-sqlite3 needs cgo, and cgo breaks the cross-compilation that the
// entire distribution model rests on.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// FileName is the database file inside the data directory.
const FileName = "theia.db"

// Open opens the database at path, creating it if necessary, and brings its
// schema up to date.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	// WAL lets a scan write while the API reads, which is the normal state of
	// affairs here. busy_timeout turns the occasional lock collision into a
	// short wait instead of an immediate "database is locked". foreign_keys is
	// off by default in SQLite and has to be asked for.
	dsn := "file:" + url.PathEscape(filepath.ToSlash(path)) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)"

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("opening database at %s: %w", path, err)
	}

	if err := Migrate(ctx, database); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

// Migrate applies every migration that has not run yet, in filename order.
// Each one runs in its own transaction, so a failure leaves the database at the
// last version that fully applied rather than half-way through one.
func Migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("creating the migrations table: %w", err)
	}

	applied, err := appliedMigrations(ctx, database)
	if err != nil {
		return err
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		statements, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, database, name, string(statements)); err != nil {
			return err
		}
	}
	return nil
}

func appliedMigrations(ctx context.Context, database *sql.DB) (map[string]bool, error) {
	rows, err := database.QueryContext(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("reading applied migrations: %w", err)
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func migrationNames() ([]string, error) {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("listing migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	// Filenames are zero-padded, so lexical order is application order.
	sort.Strings(names)
	return names, nil
}

func applyMigration(ctx context.Context, database *sql.DB, name, statements string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting migration %s: %w", name, err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op once Commit succeeds

	if _, err := tx.ExecContext(ctx, statements); err != nil {
		return fmt.Errorf("applying migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (name, applied_at) VALUES (?, unixepoch())`, name,
	); err != nil {
		return fmt.Errorf("recording migration %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration %s: %w", name, err)
	}
	return nil
}
