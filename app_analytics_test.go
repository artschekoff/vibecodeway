package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAnalyticsDashboardCountsTodaysActivity(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	a := &App{db: s, ctx: ctx}

	// One issue seen open, then seen done → one completion stamped "now".
	open := []appstore.BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "In Progress", IssueType: "Task", Org: "acme", Priority: "Medium", Created: "2026-08-01T09:00:00Z"},
	}
	require.NoError(t, a.db.UpsertBoardJiraAssignments(ctx, open))
	done := []appstore.BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Done", IssueType: "Task", Org: "acme", Priority: "Medium", Created: "2026-08-01T09:00:00Z"},
	}
	require.NoError(t, a.db.UpsertBoardJiraAssignments(ctx, done))

	require.NoError(t, a.db.InsertStatEvent(ctx, appstore.StatEvent{
		Kind:       "pr_merged",
		ExternalID: "pr-1",
		Org:        "acme",
		Repo:       "acme/repo",
		JiraKey:    "JH-1",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	}))

	dashboard, err := a.GetAnalyticsDashboard(30)
	require.NoError(t, err)
	assert.Equal(t, 30, dashboard.RangeDays)
	assert.Len(t, dashboard.Daily, 30)
	assert.Equal(t, time.Now().Format("2006-01-02"), dashboard.Daily[29].Day)
	assert.Equal(t, 1, dashboard.Daily[29].Tasks)
	assert.Equal(t, 1, dashboard.Daily[29].PRs)
	if assert.Len(t, dashboard.ByType, 1) {
		assert.Equal(t, "Task", dashboard.ByType[0].Label)
		assert.Equal(t, 1, dashboard.ByType[0].Count)
	}
}
