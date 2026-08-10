package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/jira"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type stubCommentLister struct {
	calls atomic.Int32
}

func (s *stubCommentLister) ListComments(_ context.Context, _ string) ([]jira.Comment, error) {
	s.calls.Add(1)
	return nil, nil
}

func TestSyncIssueComments_AllIssuesFetched(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Open a real in-memory SQLite store.
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	repo := jirastore.NewRepo(s.DB())

	// Create a context and 20 real issues.
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	issues := make([]jirastore.Issue, 20)
	for i := range issues {
		upserted, err := repo.UpsertIssue(ctx, c.ID, fmt.Sprintf("PROJ-%d", i+1), "summary", "open", "", "task", "", "2024-01-01T00:00:00Z", "", "")
		require.NoError(t, err)
		issues[i] = jirastore.Issue{ID: upserted.ID, JiraKey: fmt.Sprintf("PROJ-%d", i+1)}
	}

	stub := &stubCommentLister{}
	total, err := syncIssueComments(ctx, stub, repo, issues, logger)
	require.NoError(t, err)
	assert.Equal(t, int32(20), stub.calls.Load(), "all 20 issues must be fetched via the pool")
	assert.Equal(t, 0, total, "stub returns no comments so total must be 0")
}

func TestSyncIssueComments_EmptyAndNilGuards(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)
	stub := &stubCommentLister{}

	// Empty issues slice: returns early before any API or DB calls.
	total, err := syncIssueComments(ctx, stub, nil, []jirastore.Issue{}, logger)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Equal(t, int32(0), stub.calls.Load(), "no calls for empty slice")
}
