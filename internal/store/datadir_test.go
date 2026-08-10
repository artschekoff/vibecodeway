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

	// Old dir gone; new dir has the renamed db + wal with preserved content.
	_, err := os.Stat(oldDir)
	require.True(t, os.IsNotExist(err), "old dir must be gone")
	b, err := os.ReadFile(filepath.Join(newDir, "vibecodeway.db"))
	require.NoError(t, err)
	require.Equal(t, "data", string(b))
	_, err = os.Stat(filepath.Join(newDir, "vibecodeway.db-wal"))
	require.NoError(t, err)

	// Idempotent: a second call is a no-op because newDir now exists.
	require.NoError(t, migrateDataDir(oldDir, newDir, "context_consolidator.db", "vibecodeway.db"))
}

func TestMigrateDataDirFreshInstall(t *testing.T) {
	tmp := t.TempDir()
	// Neither dir exists → no-op, no error.
	require.NoError(t, migrateDataDir(
		filepath.Join(tmp, ".context-consolidator"),
		filepath.Join(tmp, ".vibecodeway"),
		"context_consolidator.db", "vibecodeway.db"))
	_, err := os.Stat(filepath.Join(tmp, ".vibecodeway"))
	require.True(t, os.IsNotExist(err))
}
