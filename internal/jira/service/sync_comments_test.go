package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/jira"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
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

	issues := make([]jirastore.Issue, 20)
	for i := range issues {
		issues[i] = jirastore.Issue{ID: fmt.Sprintf("issue-%d", i), JiraKey: fmt.Sprintf("PROJ-%d", i+1)}
	}

	stub := &stubCommentLister{}
	total, err := syncIssueComments(ctx, stub, nil, issues, logger)
	require.NoError(t, err)
	// nil jiraRepo now causes an early return before spawning goroutines;
	// no ListComments calls are expected.
	assert.Equal(t, int32(0), stub.calls.Load(), "nil repo early-returns without fetching")
	assert.Equal(t, 0, total)
}
