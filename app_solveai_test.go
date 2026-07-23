package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/analysis"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	openaiclient "github.com/artschekoff/context-consolidator/internal/openai"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
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
	_, err := app.SolveWithAI(contextID, "реализовать")
	require.Error(t, err)
}

func TestFirstAttachedRepoPath(t *testing.T) {
	app, contextID := newAppWithContext(t)

	// No linked repo yet -> error.
	_, err := app.firstAttachedRepoPath(contextID)
	require.Error(t, err)

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoX")
	require.NoError(t, err)
	require.NoError(t, app.vcsStore.Repos.UpdateLocalPath(app.ctx, repo.ID, "/tmp/x"))

	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "main", "")
	require.NoError(t, err)

	path, err := app.firstAttachedRepoPath(contextID)
	require.NoError(t, err)
	require.Equal(t, "/tmp/x", path)
}
