package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestSaveRepoFile_rejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on windows")
	}
	dir := gitInitRepo(t)
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))

	app := &App{}
	err := app.SaveRepoFile(dir, "link/evil.txt", "x")
	require.Error(t, err)

	_, statErr := os.Stat(filepath.Join(outside, "evil.txt"))
	require.True(t, os.IsNotExist(statErr), "file must not be written outside the repo")
}

func TestSaveRepoFile_allowsNewFileInRealSubdir(t *testing.T) {
	dir := gitInitRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	app := &App{}
	require.NoError(t, app.SaveRepoFile(dir, "sub/new.txt", "x"))

	got, err := os.ReadFile(filepath.Join(dir, "sub", "new.txt"))
	require.NoError(t, err)
	require.Equal(t, "x", string(got))
}

func TestReadRepoFileHead_rejectsEscape(t *testing.T) {
	dir := gitInitRepo(t)
	app := &App{}
	head, err := app.ReadRepoFileHead(dir, "../outside")
	require.NoError(t, err)
	require.Equal(t, "", head)
}
