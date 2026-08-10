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

// migrateDataDir renames oldDir→newDir, then renames the db file (and its
// -wal/-shm siblings) inside the moved dir. No-op if newDir already exists or
// oldDir is absent.
func migrateDataDir(oldDir, newDir, oldDB, newDB string) error {
	if _, err := os.Stat(newDir); err == nil {
		return nil // already migrated, or a fresh install already created it
	}
	if _, err := os.Stat(oldDir); err != nil {
		return nil // nothing to migrate
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("migrate data dir: %w", err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		oldP := filepath.Join(newDir, oldDB+suffix)
		newP := filepath.Join(newDir, newDB+suffix)
		if _, err := os.Stat(oldP); err == nil {
			if err := os.Rename(oldP, newP); err != nil {
				return fmt.Errorf("migrate db file %s: %w", oldP, err)
			}
		}
	}
	return nil
}
