package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("s\n"), 0o644))
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	return dir
}

func TestStageFile_stagesUntracked(t *testing.T) {
	a := &App{}
	dir := initGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("n\n"), 0o644))

	require.NoError(t, a.StageFile(dir, "new.txt"))

	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "new.txt").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "A  new.txt")
}

func TestDiscardFile_removesUntracked(t *testing.T) {
	a := &App{}
	dir := initGitRepo(t)
	p := filepath.Join(dir, "junk.txt")
	require.NoError(t, os.WriteFile(p, []byte("j\n"), 0o644))

	require.NoError(t, a.DiscardFile(dir, "junk.txt"))

	_, statErr := os.Stat(p)
	assert.True(t, os.IsNotExist(statErr), "untracked file should be deleted")
}

func TestDiscardFile_rejectsPathEscape(t *testing.T) {
	a := &App{}
	dir := initGitRepo(t)
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("keep\n"), 0o644))

	err := a.DiscardFile(dir, "../outside.txt")
	require.Error(t, err)
	_, statErr := os.Stat(outside)
	assert.NoError(t, statErr, "file outside the repo must not be deleted")
}
