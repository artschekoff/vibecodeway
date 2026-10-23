package main

import (
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_MyAssignedJiraIssues_listMarkAndBoard(t *testing.T) {
	a := newAutoLinkTestApp(t) // *App with a real store; see app_autolink_test.go

	// Seed the store directly (Refresh needs live acli, not available in tests).
	require.NoError(t, a.db.UpsertBoardJiraAssignments(a.ctx, []appstore.BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Open", IssueType: "Task", Priority: "High", Created: "2026-07-20T10:00:00Z"},
	}))

	list, err := a.ListMyAssignedJiraIssues("priority")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Seen)

	// GetMyBoard surfaces the same list.
	board, err := a.GetMyBoard("")
	require.NoError(t, err)
	require.Len(t, board.MyAssignedJiraIssues, 1)
	assert.Equal(t, "JH-1", board.MyAssignedJiraIssues[0].Key)

	// Mark seen flips the flag.
	require.NoError(t, a.MarkMyAssignedJiraIssuesSeen())
	list, err = a.ListMyAssignedJiraIssues("priority")
	require.NoError(t, err)
	assert.True(t, list[0].Seen)
}

func TestIsSubtaskType(t *testing.T) {
	assert.True(t, isSubtaskType("Subtask"))
	assert.True(t, isSubtaskType("Sub-task"))
	assert.True(t, isSubtaskType("sub task"))
	assert.False(t, isSubtaskType("Task"))
	assert.False(t, isSubtaskType("Story"))
}

func TestSubtaskParentCache(t *testing.T) {
	a := newAutoLinkTestApp(t)
	require.NoError(t, a.db.SetSubtaskParent(a.ctx, "JH-2", "JH-1"))
	require.NoError(t, a.db.SetSubtaskParent(a.ctx, "JH-2", "JH-9")) // upsert wins
	m, err := a.db.GetSubtaskParents(a.ctx, []string{"JH-2", "JH-404"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"JH-2": "JH-9"}, m)
}
