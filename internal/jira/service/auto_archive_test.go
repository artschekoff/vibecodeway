package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// After a refresh, a context whose Jira tickets are all terminal is auto-archived
// (deactivated); contexts with any open ticket, or no tickets at all, stay active.
func TestRefreshContext_autoArchivesWhenAllTicketsDone(t *testing.T) {
	svc, s := newServiceWithJira(t, nil)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)

	doneCtx, err := s.CreateContext(ctx, "DONE-1", org.ID)
	require.NoError(t, err)
	insertIssueStatus(t, svc, doneCtx.ID, "JH-1", "Done")
	insertIssueStatus(t, svc, doneCtx.ID, "JH-2", "Closed")

	openCtx, err := s.CreateContext(ctx, "OPEN-1", org.ID)
	require.NoError(t, err)
	insertIssueStatus(t, svc, openCtx.ID, "JH-3", "Done")
	insertIssueStatus(t, svc, openCtx.ID, "JH-4", "In Progress")

	emptyCtx, err := s.CreateContext(ctx, "EMPTY-1", org.ID)
	require.NoError(t, err)

	for _, id := range []string{doneCtx.ID, openCtx.ID, emptyCtx.ID} {
		_, err := svc.RefreshContext(ctx, id)
		require.NoError(t, err)
	}

	got, err := s.GetContext(ctx, doneCtx.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", got.Status, "all-done context should auto-archive")

	got, err = s.GetContext(ctx, openCtx.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", got.Status, "context with an open ticket stays active")

	got, err = s.GetContext(ctx, emptyCtx.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", got.Status, "context with no tickets stays active")
}

func insertIssueStatus(t *testing.T, svc *ContextService, ctxID, key, status string) {
	t.Helper()
	_, err := svc.store.DB().ExecContext(context.Background(),
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES (?, ?, ?, 's', ?, '', datetime('now'))`, key+"-"+ctxID, ctxID, key, status)
	require.NoError(t, err)
}
