package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A checkout of the branch made elsewhere (an agent's /tmp worktree) must never
// become the workspace: the app creates its own under <clone>/.worktrees even
// when the branch is already checked out there, and an empty branch yields nothing.
func TestEnsureWorktreeForBranch_ignoresForeignWorktrees(t *testing.T) {
	dir := initWorktreeTestRepo(t)
	s, err := appstore.Open(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	vs := vcsstore.NewStore(s.DB())
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "r")
	require.NoError(t, err)
	rp, err := vs.RepoPaths.Add(ctx, repo.ID, dir)
	require.NoError(t, err)
	a := &App{vcsStore: vs, ctx: ctx, logger: newTestLogger()}

	// Someone checked feat/x out in a temp dir and a stale DB row points there.
	foreign := filepath.Join(t.TempDir(), "agent-checkout")
	require.NoError(t, exec.Command("git", "-C", dir, "worktree", "add", foreign, "feat/x").Run())
	require.NoError(t, vs.Workspaces.Create(ctx, &vcsstore.VCSWorkspace{
		RepoPathID: rp.ID, RepoID: repo.ID, Branch: "feat/x", WorktreePath: foreign, Status: "active",
	}))

	path, err := a.ensureWorktreeForBranch(repo.ID, "feat/x")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, ".worktrees", "feat-x"), path, "workspace lives under the registered clone")

	ws, err := vs.Workspaces.FindByBranch(ctx, repo.ID, "feat/x")
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, path, ws.WorktreePath, "the foreign record was replaced")

	empty, err := a.ensureWorktreeForBranch(repo.ID, "")
	require.NoError(t, err)
	assert.Equal(t, "", empty)
}
