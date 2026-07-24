package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func gitInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		require.NoError(t, exec.Command("git", append([]string{"-C", dir}, args...)...).Run())
	}
	return dir
}

func TestReadRepoFileHead_committedAndUntracked(t *testing.T) {
	dir := gitInitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v1\n"), 0o644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", "a.txt").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "c").Run())
	// Modify working copy.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v2\n"), 0o644))
	// Untracked file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new\n"), 0o644))

	app := &App{}
	head, err := app.ReadRepoFileHead(dir, "a.txt")
	require.NoError(t, err)
	require.Equal(t, "v1\n", head)

	work, err := app.ReadRepoFile(dir, "a.txt")
	require.NoError(t, err)
	require.Equal(t, "v2\n", work)

	untrackedHead, err := app.ReadRepoFileHead(dir, "b.txt")
	require.NoError(t, err)
	require.Equal(t, "", untrackedHead)
}

func TestSaveRepoFile(t *testing.T) {
	dir := gitInitRepo(t)
	app := &App{}
	require.NoError(t, app.SaveRepoFile(dir, "c.txt", "saved\n"))
	got, err := os.ReadFile(filepath.Join(dir, "c.txt"))
	require.NoError(t, err)
	require.Equal(t, "saved\n", string(got))
}

func TestSaveRepoFile_rejectsEscape(t *testing.T) {
	dir := gitInitRepo(t)
	app := &App{}
	require.Error(t, app.SaveRepoFile(dir, "../evil.txt", "x"))
}
