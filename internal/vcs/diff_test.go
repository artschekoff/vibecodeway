package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	require.NoError(t, cmd.Run(), "git %v", args)
}

func TestRepoDiff(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-m", "init")
	// modify tracked + add untracked
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new\n"), 0o644))

	changes, err := RepoDiff(dir)
	require.NoError(t, err)

	byPath := map[string]FileChange{}
	for _, c := range changes {
		byPath[c.Path] = c
	}
	require.Contains(t, byPath, "a.txt")
	assert.Equal(t, "modified", byPath["a.txt"].Status)
	assert.Contains(t, byPath["a.txt"].Diff, "+two")
	require.Contains(t, byPath, "b.txt")
	assert.Equal(t, "untracked", byPath["b.txt"].Status)
}
