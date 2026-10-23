package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/analysis"
	jiraservice "github.com/artschekoff/vibecodeway/internal/jira/service"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	openaiclient "github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newAppWithContext builds an App backed by a temp SQLite DB, wired up with
// just enough services (db, vcsStore, analysisSvc) to exercise SolveWithAI
// and firstAttachedRepoPath, and seeds one org + one context with no linked
// repo.
func newAppWithContext(t *testing.T) (*App, string) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	ctxRow, err := s.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	logger := zap.NewNop()
	jiraRepo := jirastore.NewRepo(s.DB())
	jiraSvc := jiraservice.NewContextService(s, nil, jiraRepo, nil, nil, logger)

	vs := vcsstore.NewStore(s.DB())

	openAI := openaiclient.NewClient(s, logger)
	analysisSvc := analysis.NewService(s, jiraSvc, openAI, logger)
	analysisSvc.SetVCSStore(vs)

	app := &App{
		ctx:         ctx,
		db:          s,
		vcsStore:    vs,
		analysisSvc: analysisSvc,
		logger:      logger,
	}

	return app, ctxRow.ID
}

func TestSolveWithAI_noRepo_errors(t *testing.T) {
	app, contextID := newAppWithContext(t)
	_, err := app.SolveWithAI(contextID, "реализовать", "", "")
	require.Error(t, err, "a target must be picked first")

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "nopath")
	require.NoError(t, err)
	_, err = app.SolveWithAI(contextID, "реализовать", repo.ID, "feat/x")
	require.Error(t, err, "no local checkout on disk")
}

// Solve with AI checks out only the picked branch: the other repo attached to
// the context gets no worktree (and so no on-checkout script run).
func TestSolveWithAI_checksOutOnlyPickedBranch(t *testing.T) {
	app, contextID := newAppWithContext(t)
	dirA, dirB := initWorktreeTestRepo(t), initWorktreeTestRepo(t)
	repoA, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "api")
	require.NoError(t, err)
	repoB, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "rdbms")
	require.NoError(t, err)
	for _, x := range []struct{ id, dir string }{{repoA.ID, dirA}, {repoB.ID, dirB}} {
		_, err = app.vcsStore.RepoPaths.Add(app.ctx, x.id, x.dir)
		require.NoError(t, err)
		_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, x.id, "feat/x", "", true)
		require.NoError(t, err)
	}

	res, err := app.SolveWithAI(contextID, "реализовать", repoA.ID, "feat/x")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dirA, ".worktrees", "feat-x"), res.WorktreePath)
	require.NotEmpty(t, res.Instruction)

	wsB, err := app.FindWorkspaceForBranch(repoB.ID, "feat/x")
	require.NoError(t, err)
	require.Nil(t, wsB, "the unpicked repo stays untouched")
}

func TestFirstAttachedRepoPath(t *testing.T) {
	app, contextID := newAppWithContext(t)

	// No linked repo yet -> error.
	_, err := app.firstAttachedRepoPath(contextID)
	require.Error(t, err)

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoX")
	require.NoError(t, err)
	require.NoError(t, app.vcsStore.Repos.UpdateLocalPath(app.ctx, repo.ID, "/tmp/x"))

	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "main", "", true)
	require.NoError(t, err)

	path, err := app.firstAttachedRepoPath(contextID)
	require.NoError(t, err)
	require.Equal(t, "/tmp/x", path)
}

// The real app stores local paths in vcs_repo_paths (via RepoPaths.Add from the
// UI), NOT in the legacy vcs_repos.local_path column — which stays empty. This
// reproduces the "no linked repo with a local path found" error users hit even
// though their repos clearly have paths configured.
func TestFirstAttachedRepoPath_usesRepoPaths(t *testing.T) {
	app, contextID := newAppWithContext(t)

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoY")
	require.NoError(t, err)
	// Path configured only in vcs_repo_paths; local_path left empty (as in prod).
	_, err = app.vcsStore.RepoPaths.Add(app.ctx, repo.ID, "/tmp/y")
	require.NoError(t, err)

	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "main", "", true)
	require.NoError(t, err)

	path, err := app.firstAttachedRepoPath(contextID)
	require.NoError(t, err)
	require.Equal(t, "/tmp/y", path)
}
