package workflow_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/artschekoff/context-consolidator/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCompute_NoBranches(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, _ := s.CreateOrganization(ctx, "test-org")
	c, _ := s.CreateContext(ctx, "no-branch-ctx", org.ID)
	status, comment, err := workflow.Compute(ctx, c.ID, s, vcsstore.NewStore(s.DB()))
	require.NoError(t, err)
	assert.Equal(t, "not_started", status)
	assert.Empty(t, comment)
}

func TestCompute_OpenQuestionsGivesScoping(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	org, _ := s.CreateOrganization(ctx, "test-org")
	c, _ := s.CreateContext(ctx, "oq-ctx", org.ID)
	vs := vcsstore.NewStore(s.DB())
	repo, _ := vs.Repos.EnsureRepo(ctx, "github", "owner", "repo")
	_, _ = vs.Branches.AddBranch(ctx, c.ID, repo.ID, "main", "")
	_, _ = s.CreateOpenQuestion(ctx, c.ID, "What is the deadline?", "ai")

	status, _, err := workflow.Compute(ctx, c.ID, s, vs)
	require.NoError(t, err)
	assert.Equal(t, "scoping", status)
}
