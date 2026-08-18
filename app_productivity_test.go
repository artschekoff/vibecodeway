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

func TestGetProductivityStatsCountsTodaysCompletions(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	a := &App{db: s, ctx: ctx}

	// Two issues seen open, then seen done → two completions stamped "now".
	open := []appstore.BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "In Progress", IssueType: "Task", Priority: "Medium", Created: "2026-08-01T09:00:00Z"},
		{Key: "JH-2", Summary: "two", Status: "In Progress", IssueType: "Task", Priority: "Medium", Created: "2026-08-01T09:00:00Z"},
	}
	require.NoError(t, a.db.UpsertBoardJiraAssignments(ctx, open))
	done := []appstore.BoardJiraAssignment{
		{Key: "JH-1", Summary: "one", Status: "Done", IssueType: "Task", Priority: "Medium", Created: "2026-08-01T09:00:00Z"},
		{Key: "JH-2", Summary: "two", Status: "Done", IssueType: "Task", Priority: "Medium", Created: "2026-08-01T09:00:00Z"},
	}
	require.NoError(t, a.db.UpsertBoardJiraAssignments(ctx, done))

	stats, err := a.GetProductivityStats()
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Today)
	assert.Equal(t, 8.0, stats.WorkdayHours)
	assert.Equal(t, 2.0, stats.TaskHours)
	assert.Equal(t, 4.0, stats.HoursToday)
	assert.Equal(t, 2, stats.TasksToNorm)
	assert.Len(t, stats.Daily, 30)
	assert.Equal(t, time.Now().Format("2006-01-02"), stats.Daily[29].Day)
}

func TestSetProductivitySettingsClampsToBounds(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	a := &App{db: s, ctx: ctx}

	// Non-positive values clamp to the floor, not the old 8h/2h fallback.
	require.NoError(t, a.SetProductivitySettings(0, -1))
	got, err := a.db.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0.5, got.WorkdayHours)
	assert.Equal(t, 0.25, got.TaskHours)

	// Very large values clamp to the ceiling so the headline can't blow up
	// (e.g. workdayHours = 1e9 must not survive into tasksToNorm math).
	require.NoError(t, a.SetProductivitySettings(1e9, 1e9))
	got, err = a.db.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 24.0, got.WorkdayHours)
	assert.Equal(t, 24.0, got.TaskHours)
}

// SetAppSettings only owns the terminal/refetch fields — saving those must not
// wipe unrelated persisted settings like WorkdayHours/TaskHours (or, previously,
// EditorSettings). Regression test for the "fresh struct literal" data-loss bug.
func TestSetAppSettingsPreservesProductivitySettings(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	a := &App{db: s, ctx: ctx}

	require.NoError(t, a.SetProductivitySettings(6.5, 1.5))

	// Unrelated save: terminal app + refetch settings, both refetch flags off so
	// restartBackgroundRefresh doesn't spin up a goroutine that needs a.logger.
	require.NoError(t, a.SetAppSettings(false, 15, false, 15, "iTerm", "cursor"))

	got, err := a.db.GetAppSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6.5, got.WorkdayHours)
	assert.Equal(t, 1.5, got.TaskHours)
	assert.Equal(t, "iTerm", got.TerminalAppPath)
}
