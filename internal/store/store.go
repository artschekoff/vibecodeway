// Package store provides the shared SQLite database infrastructure and core context persistence.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // register sqlite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const driverName = "sqlite"

// Store wraps the SQLite database connection.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at the given path and runs migrations.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open(driverName, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return s, nil
}

// DefaultDBPath returns the default path for the SQLite database.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".context-consolidator", "context_consolidator.db")
}

// DB returns the underlying *sql.DB for use by feature-specific repositories.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func configure(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("setting pragma %q: %w", p, err)
		}
	}
	return nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version := entry.Name()

		var exists int
		if err := s.db.QueryRow(
			"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version,
		).Scan(&exists); err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if exists > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + version)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", version, err)
		}

		up, err := extractUpSection(string(content))
		if err != nil {
			return fmt.Errorf("parsing migration %s: %w", version, err)
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning migration tx: %w", err)
		}

		if _, err := tx.Exec(up); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("executing migration %s: %w", version, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, datetime('now'))",
			version,
		); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("recording migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", version, err)
		}
	}
	return nil
}

// extractUpSection pulls the SQL between "-- +goose Up" and "-- +goose Down".
func extractUpSection(content string) (string, error) {
	const upMarker = "-- +goose Up"
	const downMarker = "-- +goose Down"

	start := strings.Index(content, upMarker)
	if start < 0 {
		return "", fmt.Errorf("missing %q marker", upMarker)
	}
	start += len(upMarker)

	end := strings.Index(content[start:], downMarker)
	if end < 0 {
		return content[start:], nil
	}
	return content[start : start+end], nil
}
