package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateDataDir(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, ".context-consolidator")
	newDir := filepath.Join(tmp, ".vibecodeway")
	require.NoError(t, os.MkdirAll(oldDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "context_consolidator.db"), []byte("data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "context_consolidator.db-wal"), []byte("wal"), 0o644))

	require.NoError(t, migrateDataDir(oldDir, newDir, "context_consolidator.db", "vibecodeway.db"))

	// New dir has the renamed db + wal with preserved content.
	b, err := os.ReadFile(filepath.Join(newDir, "vibecodeway.db"))
	require.NoError(t, err)
	require.Equal(t, "data", string(b))
	_, err = os.Stat(filepath.Join(newDir, "vibecodeway.db-wal"))
	require.NoError(t, err)

	// Old db file was moved (no longer present in oldDir).
	_, err = os.Stat(filepath.Join(oldDir, "context_consolidator.db"))
	require.True(t, os.IsNotExist(err), "old db file must be gone")

	// Idempotent: a second call is a no-op because the new db now exists.
	require.NoError(t, migrateDataDir(oldDir, newDir, "context_consolidator.db", "vibecodeway.db"))
}

// TestMigrateDataDirNewDirPreexists reproduces the bug where newDir is created
// by an earlier startup step (e.g. logger setup creating the data dir before
// migration runs) before migrateDataDir gets a chance to run. The migration must
// still find and move the old DB, keying off the DB file rather than the dir.
func TestMigrateDataDirNewDirPreexists(t *testing.T) {
	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, ".context-consolidator")
	newDir := filepath.Join(tmp, ".vibecodeway")

	// newDir is created FIRST (simulating buildLogger() running before migration)
	// with an unrelated stray file already in it.
	require.NoError(t, os.MkdirAll(newDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(newDir, "app.log"), []byte("log"), 0o644))

	// oldDir is created afterward with the legacy db to migrate.
	require.NoError(t, os.MkdirAll(oldDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "context_consolidator.db"), []byte("data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "context_consolidator.db-wal"), []byte("wal"), 0o644))

	require.NoError(t, migrateDataDir(oldDir, newDir, "context_consolidator.db", "vibecodeway.db"))

	// The db was migrated into the pre-existing newDir.
	b, err := os.ReadFile(filepath.Join(newDir, "vibecodeway.db"))
	require.NoError(t, err)
	require.Equal(t, "data", string(b))

	// The stray file that was already in newDir is untouched.
	_, err = os.Stat(filepath.Join(newDir, "app.log"))
	require.NoError(t, err, "pre-existing file in newDir must survive migration")
}

func TestMigrateDataDirFreshInstall(t *testing.T) {
	tmp := t.TempDir()
	// Neither dir exists → no-op, no error.
	require.NoError(t, migrateDataDir(
		filepath.Join(tmp, ".context-consolidator"),
		filepath.Join(tmp, ".vibecodeway"),
		"context_consolidator.db", "vibecodeway.db",
	))
	_, err := os.Stat(filepath.Join(tmp, ".vibecodeway"))
	require.True(t, os.IsNotExist(err))
}
