package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assignment(key, status string) BoardJiraAssignment {
	return BoardJiraAssignment{
		Key:       key,
		Summary:   "summary of " + key,
		Status:    status,
		IssueType: "Task",
		Priority:  "Medium",
		Created:   "2026-08-01T09:00:00Z",
		Org:       "juna-api",
	}
}

func TestBoardRefreshRecordsCompletionOnTransition(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{assignment("JH-1", "In Progress")}))
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{assignment("JH-1", "Done")}))

	got, err := s.ListTaskCompletionsSince(ctx, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "JH-1", got[0].Key)
	assert.Equal(t, "Task", got[0].IssueType)
	assert.Equal(t, "juna-api", got[0].Org)
	assert.Equal(t, "Done", got[0].Status)
	assert.NotEmpty(t, got[0].CompletedAt)
}

func TestBoardRefreshIgnoresFirstSeenDone(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// An issue that is already closed the first time we ever see it has no
	// observed transition — counting it would inflate the day it was imported.
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{assignment("JH-2", "Done")}))

	got, err := s.ListTaskCompletionsSince(ctx, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestBoardRefreshDoesNotDoubleCountRepeatedDone(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{assignment("JH-3", "To Do")}))
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{assignment("JH-3", "Closed")}))
	require.NoError(t, s.UpsertBoardJiraAssignments(ctx, []BoardJiraAssignment{assignment("JH-3", "Closed")}))

	got, err := s.ListTaskCompletionsSince(ctx, "2000-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestListTaskCompletionsSinceFiltersOlderRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_completions (jira_key, issue_type, org, status, completed_at)
		VALUES ('OLD-1', 'Task', '', 'Done', '2026-01-01T10:00:00Z'),
		       ('NEW-1', 'Bug',  '', 'Done', '2026-08-15T10:00:00Z')`)
	require.NoError(t, err)

	got, err := s.ListTaskCompletionsSince(ctx, "2026-08-01T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "NEW-1", got[0].Key)
}
