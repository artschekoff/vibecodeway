package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initWorktreeTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	run("init", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x\n"), 0o644))
	run("add", ".")
	run("commit", "-m", "init")
	run("branch", "feat/x")
	return dir
}

func TestEnsureWorktreeForBranch_createsAndIsIdempotent(t *testing.T) {
	dir := initWorktreeTestRepo(t)
	s, err := appstore.Open(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	vs := vcsstore.NewStore(s.DB())
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "r")
	require.NoError(t, err)
	_, err = vs.RepoPaths.Add(ctx, repo.ID, dir)
	require.NoError(t, err)

	a := &App{vcsStore: vs, ctx: ctx, logger: newTestLogger()}

	path, err := a.ensureWorktreeForBranch(repo.ID, "feat/x")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".worktrees", "feat-x"), path)
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "worktree directory should exist on disk")

	// Idempotent: a second call finds the recorded workspace and returns the same path.
	again, err := a.ensureWorktreeForBranch(repo.ID, "feat/x")
	require.NoError(t, err)
	assert.Equal(t, path, again)
}

func TestEnsureWorktreeForBranch_noLocalPathReturnsEmpty(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	vs := vcsstore.NewStore(s.DB())
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "nopaths")
	require.NoError(t, err)

	a := &App{vcsStore: vs, ctx: ctx, logger: newTestLogger()}
	path, err := a.ensureWorktreeForBranch(repo.ID, "main")
	require.NoError(t, err)
	assert.Equal(t, "", path)
}
