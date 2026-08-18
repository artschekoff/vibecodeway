package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A context with a named branch + a local path gets its worktree auto-created,
// so the editor is ready to open with no picker.
func TestEnsureContextEditorWorkspace_ready(t *testing.T) {
	app, contextID := newAppWithContext(t)
	dir := initWorktreeTestRepo(t) // main + feat/x

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoReady")
	require.NoError(t, err)
	_, err = app.vcsStore.RepoPaths.Add(app.ctx, repo.ID, dir)
	require.NoError(t, err)
	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "feat/x", "", true)
	require.NoError(t, err)

	prep, err := app.EnsureContextEditorWorkspace(contextID)
	require.NoError(t, err)
	assert.True(t, prep.Ready)
	assert.Empty(t, prep.Choices)
}

// A context that only links a repo (empty branch, "track all PRs") has no
// workspace to open — the repo is offered as a checkout choice.
func TestEnsureContextEditorWorkspace_offersChoiceWhenNoWorkspace(t *testing.T) {
	app, contextID := newAppWithContext(t)
	dir := initWorktreeTestRepo(t)

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoOnly")
	require.NoError(t, err)
	_, err = app.vcsStore.RepoPaths.Add(app.ctx, repo.ID, dir)
	require.NoError(t, err)
	// Empty branch => repo-only link, no worktree gets created.
	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "", "", true)
	require.NoError(t, err)

	prep, err := app.EnsureContextEditorWorkspace(contextID)
	require.NoError(t, err)
	assert.False(t, prep.Ready)
	require.Len(t, prep.Choices, 1)
	assert.Equal(t, "acme/repoOnly", prep.Choices[0].RepoName)
	require.Len(t, prep.Choices[0].Paths, 1)
	assert.Equal(t, dir, prep.Choices[0].Paths[0].Path)
}

// A context linking a repo with no local path can neither open nor offer a
// checkout — Ready is false with no choices (frontend shows an error).
func TestEnsureContextEditorWorkspace_noLocalPath(t *testing.T) {
	app, contextID := newAppWithContext(t)

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoNoPath")
	require.NoError(t, err)
	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "feat/y", "", true)
	require.NoError(t, err)

	prep, err := app.EnsureContextEditorWorkspace(contextID)
	require.NoError(t, err)
	assert.False(t, prep.Ready)
	assert.Empty(t, prep.Choices)
}

// Two linked repos each with a named branch => two ready worktrees returned,
// so the frontend can offer a choice.
func TestEnsureContextEditorWorkspace_returnsAllWorktrees(t *testing.T) {
	app, contextID := newAppWithContext(t)
	dirA := initWorktreeTestRepo(t) // main + feat/x
	dirB := initWorktreeTestRepo(t)

	repoA, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoA")
	require.NoError(t, err)
	_, err = app.vcsStore.RepoPaths.Add(app.ctx, repoA.ID, dirA)
	require.NoError(t, err)
	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repoA.ID, "feat/x", "", true)
	require.NoError(t, err)

	repoB, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoB")
	require.NoError(t, err)
	_, err = app.vcsStore.RepoPaths.Add(app.ctx, repoB.ID, dirB)
	require.NoError(t, err)
	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repoB.ID, "feat/x", "", true)
	require.NoError(t, err)

	prep, err := app.EnsureContextEditorWorkspace(contextID)
	require.NoError(t, err)
	assert.True(t, prep.Ready)
	require.Len(t, prep.Worktrees, 2)

	byName := map[string]EditorWorktree{}
	for _, w := range prep.Worktrees {
		byName[w.RepoName] = w
	}
	require.Contains(t, byName, "acme/repoA")
	require.Contains(t, byName, "acme/repoB")
	assert.Equal(t, "feat/x", byName["acme/repoA"].Branch)
	assert.NotEmpty(t, byName["acme/repoA"].WorktreePath)
	assert.NotEqual(t, dirA, byName["acme/repoA"].WorktreePath) // it's the .worktrees path, not the main checkout
}
