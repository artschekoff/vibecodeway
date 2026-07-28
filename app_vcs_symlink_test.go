package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// git worktree list reports canonical (symlink-resolved) paths. When the repo's
// registered local path is itself a symlink, a naive `wt.Path == basePath`
// comparison never matches the main checkout, so it gets mistaken for a separate
// worktree. FindWorkspaceForBranch must compare paths symlink-aware and return
// nil for a branch that only lives in the main checkout.
func TestFindWorkspaceForBranch_ignoresMainCheckoutUnderSymlink(t *testing.T) {
	real := initWorktreeTestRepo(t) // main (HEAD) + feat/x, no extra worktrees
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(real, link))

	s, err := appstore.Open(filepath.Join(t.TempDir(), "app.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	vs := vcsstore.NewStore(s.DB())
	ctx := context.Background()

	repo, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "r")
	require.NoError(t, err)
	// Register the SYMLINKED path; git will report the resolved real path.
	_, err = vs.RepoPaths.Add(ctx, repo.ID, link)
	require.NoError(t, err)

	a := &App{vcsStore: vs, ctx: ctx, logger: newTestLogger()}

	ws, err := a.FindWorkspaceForBranch(repo.ID, "main")
	require.NoError(t, err)
	assert.Nil(t, ws, "the main checkout must not be reported as a worktree for its own branch")
}
