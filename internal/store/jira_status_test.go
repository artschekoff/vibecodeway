package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTerminalTicketStatus(t *testing.T) {
	assert.True(t, IsTerminalTicketStatus("Done"))
	assert.True(t, IsTerminalTicketStatus(" closed "))
	assert.True(t, IsTerminalTicketStatus("WON'T DO"))
	assert.False(t, IsTerminalTicketStatus("In Progress"))
	assert.False(t, IsTerminalTicketStatus("Ready for QA")) // downstream, not terminal
	assert.False(t, IsTerminalTicketStatus(""))
}

func TestContextTicketState(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)

	// Context A: mixed -> not all done, 1 open ticket.
	a, err := s.CreateContext(ctx, "CTX-A", org.ID)
	require.NoError(t, err)
	insertJiraIssue(t, s, "a1", a.ID, "JH-1", "Done")
	insertJiraIssue(t, s, "a2", a.ID, "JH-2", "In Progress")

	// Context B: all terminal -> all done, 0 open.
	b, err := s.CreateContext(ctx, "CTX-B", org.ID)
	require.NoError(t, err)
	insertJiraIssue(t, s, "b1", b.ID, "JH-3", "Done")
	insertJiraIssue(t, s, "b2", b.ID, "JH-4", "Closed")

	// Context C: zero tickets -> never "all done" (vacuous guard).
	c, err := s.CreateContext(ctx, "CTX-C", org.ID)
	require.NoError(t, err)

	openA, err := s.CountOpenTickets(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, openA)
	doneA, err := s.ContextAllTicketsDone(ctx, a.ID)
	require.NoError(t, err)
	assert.False(t, doneA)

	openB, err := s.CountOpenTickets(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, openB)
	doneB, err := s.ContextAllTicketsDone(ctx, b.ID)
	require.NoError(t, err)
	assert.True(t, doneB)

	openC, err := s.CountOpenTickets(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, openC)
	doneC, err := s.ContextAllTicketsDone(ctx, c.ID)
	require.NoError(t, err)
	assert.False(t, doneC)
}

func TestListKnownJiraStatuses(t *testing.T) {
	s := newTodoTestStore(t)
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "org")
	require.NoError(t, err)

	// Statuses spread over two contexts: the list is workflow-wide, not per-context.
	a, err := s.CreateContext(ctx, "CTX-A", org.ID)
	require.NoError(t, err)
	b, err := s.CreateContext(ctx, "CTX-B", org.ID)
	require.NoError(t, err)
	insertJiraIssue(t, s, "a1", a.ID, "JH-1", "To Do")
	insertJiraIssue(t, s, "a2", a.ID, "JH-2", "In Code Review")
	insertJiraIssue(t, s, "a3", a.ID, "JH-3", "To Do") // duplicate collapses
	insertJiraIssue(t, s, "b1", b.ID, "JH-4", "Ready for QA")
	insertJiraIssue(t, s, "b2", b.ID, "JH-5", "") // blank is not a status

	got, err := s.ListKnownJiraStatuses(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"In Code Review", "Ready for QA", "To Do"}, got)
}

func TestListKnownJiraStatuses_emptyWhenNoIssues(t *testing.T) {
	s := newTodoTestStore(t)
	got, err := s.ListKnownJiraStatuses(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func insertJiraIssue(t *testing.T, s *Store, id, ctxID, key, status string) {
	t.Helper()
	_, err := s.DB().ExecContext(context.Background(),
		`INSERT INTO jira_issues (id, context_id, jira_key, summary, status, jira_updated, created_at)
		 VALUES (?, ?, ?, 's', ?, '', datetime('now'))`, id, ctxID, key, status)
	require.NoError(t, err)
}
