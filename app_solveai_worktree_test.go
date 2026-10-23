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

func TestEnsureWorktreeForBranch_reusesMainCheckout(t *testing.T) {
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

	// "main" is already checked out in the clone itself: reuse it, don't
	// create a worktree copy (git would refuse anyway).
	path, err := a.ensureWorktreeForBranch(repo.ID, "main")
	require.NoError(t, err)
	assert.Equal(t, dir, path)
	_, statErr := os.Stat(filepath.Join(dir, ".worktrees"))
	assert.True(t, os.IsNotExist(statErr), "no worktree copy should be created")
	// And no workspace row is persisted for the main checkout (it switches branches).
	ws, err := vs.Workspaces.FindByBranch(ctx, repo.ID, "main")
	require.NoError(t, err)
	assert.Nil(t, ws)
}

func TestEnsureWorktreeForBranch_recreatesAfterWorktreeDeleted(t *testing.T) {
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

	// Simulate the user deleting the worktree on disk; the DB row is now stale.
	require.NoError(t, os.RemoveAll(path))
	cmd := exec.Command("git", "-C", dir, "worktree", "prune")
	require.NoError(t, cmd.Run())

	// A stale record must not be returned as-is (the editor would open into a
	// nonexistent directory); the worktree is rebuilt instead.
	again, err := a.ensureWorktreeForBranch(repo.ID, "feat/x")
	require.NoError(t, err)
	assert.Equal(t, path, again)
	_, statErr := os.Stat(again)
	assert.NoError(t, statErr, "worktree should exist on disk again")
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
