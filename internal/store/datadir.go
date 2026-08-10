package store

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	dataDirName = ".vibecodeway"
	dbFileName  = "vibecodeway.db"

	legacyDataDirName = ".context-consolidator"
	legacyDBFileName  = "context_consolidator.db"
)

// MigrateLegacyDataDir performs a one-time move of the pre-rename data directory
// (~/.context-consolidator with context_consolidator.db*) to the new location
// (~/.vibecodeway with vibecodeway.db*). Best-effort and idempotent: it is a
// no-op once the new dir exists or when there is nothing to migrate. Call this
// BEFORE Open(DefaultDBPath()).
func MigrateLegacyDataDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil // ponytail: cannot resolve home → skip; Open will create fresh under cwd fallback
	}
	return migrateDataDir(
		filepath.Join(home, legacyDataDirName),
		filepath.Join(home, dataDirName),
		legacyDBFileName, dbFileName,
	)
}

// migrateDataDir moves the pre-rename SQLite DB (and its -wal/-shm siblings) from
// oldDir/oldDB* to newDir/newDB*. It keys off the DB FILE, not the directory,
// because other startup steps (e.g. logging) may create newDir first. No-op when
// the new DB already exists (already migrated) or the old DB is absent (fresh install).
func migrateDataDir(oldDir, newDir, oldDB, newDB string) error {
	newDBPath := filepath.Join(newDir, newDB)
	if _, err := os.Stat(newDBPath); err == nil {
		return nil // already migrated
	}
	oldDBPath := filepath.Join(oldDir, oldDB)
	if _, err := os.Stat(oldDBPath); err != nil {
		return nil // nothing to migrate (fresh install)
	}
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := filepath.Join(oldDir, oldDB+suffix)
		dst := filepath.Join(newDir, newDB+suffix)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrate db file %s: %w", src, err)
		}
	}
	return nil
}
