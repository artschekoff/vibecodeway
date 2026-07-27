package vcs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stagedPaths(t *testing.T, dir string) map[string]string {
	t.Helper()
	changes, err := RepoDiff(dir)
	require.NoError(t, err)
	m := map[string]string{}
	for _, c := range changes {
		m[c.Path] = c.IndexStatus
	}
	return m
}

func TestGitStageFile_stagesUntracked(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644))
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-m", "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n\n"), 0o644))

	require.NoError(t, GitStageFile(dir, "new.txt"))
	assert.Equal(t, "added", stagedPaths(t, dir)["new.txt"])
}

func TestGitUnstageFile_unstages(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o644))
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-m", "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\ny\n"), 0o644))
	git(t, dir, "add", "a.txt")
	require.Equal(t, "modified", stagedPaths(t, dir)["a.txt"])

	require.NoError(t, GitUnstageFile(dir, "a.txt"))
	assert.Equal(t, "", stagedPaths(t, dir)["a.txt"]) // no longer staged
}

func TestGitRestoreFile_revertsTrackedAndErrorsOnUntracked(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("orig\n"), 0o644))
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-m", "init")

	// tracked change -> restore brings it back to HEAD
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("edited\n"), 0o644))
	require.NoError(t, GitRestoreFile(dir, "a.txt"))
	b, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "orig\n", string(b))

	// untracked file has no HEAD entry -> restore errors (caller falls back to delete)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "u.txt"), []byte("u\n"), 0o644))
	assert.Error(t, GitRestoreFile(dir, "u.txt"))
}
