package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/artschekoff/vibecodeway/internal/analytics"
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

// Regression test: rangeDays<=0 must clamp to analytics.DefaultRangeDays
// BEFORE the fetch window is computed, not rely on analytics.Compute's
// internal default. Fetching too small/negative a `since` window here would
// silently starve a still-30-day-wide Daily series of real data.
func TestGetAnalyticsDashboardClampsNonPositiveRangeDays(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	a := &App{db: s, ctx: ctx}

	// A completion 5 days back: with a buggy since = now-1day (or now+Nday
	// for negative rangeDays), this row would be silently excluded from
	// ListTaskCompletionsSince and never reach Daily.
	fiveDaysAgo := time.Now().UTC().AddDate(0, 0, -5)
	_, err = a.db.DB().ExecContext(ctx, `
		INSERT INTO task_completions (jira_key, issue_type, org, status, completed_at)
		VALUES ('JH-OLD', 'Task', 'acme', 'Done', ?)`,
		fiveDaysAgo.Format(time.RFC3339))
	require.NoError(t, err)

	dashboard, err := a.GetAnalyticsDashboard(0)
	require.NoError(t, err)
	assert.Equal(t, 30, dashboard.RangeDays)
	require.Len(t, dashboard.Daily, 30)

	fiveDaysAgoKey := fiveDaysAgo.Local().Format("2006-01-02")
	found := false
	for _, d := range dashboard.Daily {
		if d.Day == fiveDaysAgoKey {
			assert.Equal(t, 1, d.Tasks, "5-day-old completion should be counted, not silently dropped by a too-narrow fetch window")
			found = true
		}
	}
	assert.True(t, found, "expected day %s to be present in a 30-day Daily series", fiveDaysAgoKey)
}

// Regression test: at rangeDays=7 the fetch window must still be floored to
// cover the Month card's preceding-period comparison (up to 60 days back),
// not just 2*rangeDays+1 (=15). Otherwise the always-rendered Month card
// silently undercounts while Daily correctly stays a 7-row series.
func TestGetAnalyticsDashboardFloorsWindowForMonthCardAtSmallRangeDays(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	a := &App{db: s, ctx: ctx}

	// A completion ~40 days back: with a buggy since = now-(2*7+1)=now-15days,
	// this row would be excluded from ListTaskCompletionsSince and the Month
	// card's "prev" period (days 31-60 ago) would silently undercount it.
	fortyDaysAgo := time.Now().UTC().AddDate(0, 0, -40)
	_, err = a.db.DB().ExecContext(ctx, `
		INSERT INTO task_completions (jira_key, issue_type, org, status, completed_at)
		VALUES ('JH-OLD40', 'Task', 'acme', 'Done', ?)`,
		fortyDaysAgo.Format(time.RFC3339))
	require.NoError(t, err)

	dashboard, err := a.GetAnalyticsDashboard(7)
	require.NoError(t, err)
	assert.Equal(t, 7, dashboard.RangeDays)
	assert.Len(t, dashboard.Daily, 7, "Daily should stay a 7-row series for rangeDays=7")

	var monthCard *analytics.PeriodCard
	for i := range dashboard.Cards {
		c := &dashboard.Cards[i]
		if c.Metric == analytics.MetricTasksClosed && c.Period == analytics.PeriodMonth {
			monthCard = c
		}
	}
	require.NotNil(t, monthCard, "expected a tasksClosed/month card")
	assert.Equal(t, 1, monthCard.Prev, "month card's preceding period should count the ~40-day-old completion")
}
