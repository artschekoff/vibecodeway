package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// at builds an RFC3339 timestamp for the given local day/hour/minute, which
// is what Compute receives from the store.
func at(day time.Time, hour, minute int) string {
	y, m, d := day.Date()
	return time.Date(y, m, d, hour, minute, 0, 0, time.Local).Format(time.RFC3339)
}

func findCard(t *testing.T, cards []PeriodCard, metric, period string) PeriodCard {
	t.Helper()
	for _, c := range cards {
		if c.Metric == metric && c.Period == period {
			return c
		}
	}
	t.Fatalf("card not found: metric=%s period=%s", metric, period)
	return PeriodCard{}
}

func TestComputeDefaultsRangeDays(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	got := Compute(Input{Now: now, RangeDays: 0, TaskHours: 2})

	assert.Equal(t, DefaultRangeDays, got.RangeDays)
	assert.Len(t, got.Daily, DefaultRangeDays)
}

// TestComputePeriodBoundaryAtLocalMidnight exercises day bucketing right
// after local midnight: a completion 5 minutes before midnight must land in
// the previous calendar day, not "today", and the yesterday/prev cards must
// use calendar-day boundaries rather than a fixed elapsed-hours cutoff.
func TestComputePeriodBoundaryAtLocalMidnight(t *testing.T) {
	now := time.Date(2026, 8, 17, 0, 5, 0, 0, time.Local) // 00:05 on Aug 17
	aug16 := now.AddDate(0, 0, -1)
	aug15 := now.AddDate(0, 0, -2)

	in := Input{
		Now: now,
		Completions: []Completion{
			{JiraKey: "A-1", CompletedAt: at(aug16, 23, 55)}, // yesterday, just before midnight
			{JiraKey: "A-2", CompletedAt: at(aug15, 23, 0)},  // day before yesterday
			{JiraKey: "A-3", CompletedAt: at(now, 0, 1)},     // today, just after midnight -> excluded from cards
		},
		TaskHours: 2,
	}

	got := Compute(in)

	card := findCard(t, got.Cards, MetricTasksClosed, PeriodYesterday)
	assert.Equal(t, 1, card.Count, "the 23:55 completion belongs to Aug 16, i.e. 'yesterday'")
	assert.Equal(t, 1, card.Prev, "the Aug 15 completion is the day before yesterday")

	// Today's completion must show up in Daily but not in any card.
	require.NotEmpty(t, got.Daily)
	todayRow := got.Daily[len(got.Daily)-1]
	assert.Equal(t, "2026-08-17", todayRow.Day)
	assert.Equal(t, 1, todayRow.Tasks)
}

func TestComputeCardsDeltaPctRules(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	t.Run("both zero", func(t *testing.T) {
		got := Compute(Input{Now: now, TaskHours: 2})
		card := findCard(t, got.Cards, MetricTasksClosed, PeriodYesterday)
		assert.Equal(t, 0, card.Count)
		assert.Equal(t, 0, card.Prev)
		assert.Equal(t, 0.0, card.DeltaPct)
	})

	t.Run("prev zero, count positive", func(t *testing.T) {
		yesterday := now.AddDate(0, 0, -1)
		in := Input{
			Now: now,
			Completions: []Completion{
				{JiraKey: "A-1", CompletedAt: at(yesterday, 9, 0)},
			},
			TaskHours: 2,
		}
		got := Compute(in)
		card := findCard(t, got.Cards, MetricTasksClosed, PeriodYesterday)
		assert.Equal(t, 1, card.Count)
		assert.Equal(t, 0, card.Prev)
		assert.Equal(t, 100.0, card.DeltaPct)
	})

	t.Run("general growth", func(t *testing.T) {
		// Week: 4 tasks in the last 7 full days, 2 in the 7 before that -> +100%.
		var completions []Completion
		for i := 1; i <= 4; i++ {
			completions = append(completions, Completion{JiraKey: "W", CompletedAt: at(now.AddDate(0, 0, -i), 9, 0)})
		}
		for i := 8; i <= 9; i++ {
			completions = append(completions, Completion{JiraKey: "W", CompletedAt: at(now.AddDate(0, 0, -i), 9, 0)})
		}
		got := Compute(Input{Now: now, Completions: completions, TaskHours: 2})
		card := findCard(t, got.Cards, MetricTasksClosed, PeriodWeek)
		assert.Equal(t, 4, card.Count)
		assert.Equal(t, 2, card.Prev)
		assert.Equal(t, 100.0, card.DeltaPct)
	})
}

func TestComputeCardsOrderAndEventKinds(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1)

	in := Input{
		Now: now,
		Completions: []Completion{
			{JiraKey: "A-1", CompletedAt: at(yesterday, 9, 0)},
		},
		Events: []Event{
			{Kind: KindPRMerged, ExternalID: "pr1", OccurredAt: at(yesterday, 9, 0)},
			{Kind: KindCommentAuthored, ExternalID: "c1", OccurredAt: at(yesterday, 9, 0)},
			{Kind: KindTaskStarted, ExternalID: "t1", JiraKey: "A-1", OccurredAt: at(yesterday, 8, 0)},
		},
		TaskHours: 2,
	}

	got := Compute(in)

	require.Len(t, got.Cards, 9)
	wantOrder := []struct{ metric, period string }{
		{MetricTasksClosed, PeriodYesterday}, {MetricTasksClosed, PeriodWeek}, {MetricTasksClosed, PeriodMonth},
		{MetricPRsMerged, PeriodYesterday}, {MetricPRsMerged, PeriodWeek}, {MetricPRsMerged, PeriodMonth},
		{MetricComments, PeriodYesterday}, {MetricComments, PeriodWeek}, {MetricComments, PeriodMonth},
	}
	for i, w := range wantOrder {
		assert.Equal(t, w.metric, got.Cards[i].Metric, "card %d metric", i)
		assert.Equal(t, w.period, got.Cards[i].Period, "card %d period", i)
	}

	// task_started must never be counted as prsMerged/comments.
	prCard := findCard(t, got.Cards, MetricPRsMerged, PeriodYesterday)
	assert.Equal(t, 1, prCard.Count)
	commentCard := findCard(t, got.Cards, MetricComments, PeriodYesterday)
	assert.Equal(t, 1, commentCard.Count)
}

func TestComputeDailyLengthAndAlignment(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)
	twoDaysAgo := now.AddDate(0, 0, -2)

	in := Input{
		Now:       now,
		RangeDays: 7,
		Completions: []Completion{
			{JiraKey: "A-1", CompletedAt: at(twoDaysAgo, 9, 0)},
		},
		Events: []Event{
			{Kind: KindPRMerged, ExternalID: "pr1", OccurredAt: at(twoDaysAgo, 9, 0)},
			{Kind: KindCommentAuthored, ExternalID: "c1", OccurredAt: at(twoDaysAgo, 10, 0)},
		},
		TaskHours: 2,
	}

	got := Compute(in)

	require.Len(t, got.Daily, 7)
	assert.Equal(t, "2026-08-11", got.Daily[0].Day, "oldest day is RangeDays-1 days before today")
	assert.Equal(t, "2026-08-17", got.Daily[6].Day, "newest day is today")

	row := got.Daily[4] // 2026-08-15, two days before today
	assert.Equal(t, "2026-08-15", row.Day)
	assert.Equal(t, 1, row.Tasks)
	assert.Equal(t, 1, row.PRs)
	assert.Equal(t, 1, row.Comments)

	for i, r := range got.Daily {
		if i == 4 {
			continue
		}
		assert.Zero(t, r.Tasks, "day %s should have no tasks", r.Day)
		assert.Zero(t, r.PRs, "day %s should have no prs", r.Day)
		assert.Zero(t, r.Comments, "day %s should have no comments", r.Day)
	}
}

func TestComputeByTypeByOrgSortingAndNoneLabel(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1)

	in := Input{
		Now:       now,
		RangeDays: 30,
		Completions: []Completion{
			{JiraKey: "A-1", IssueType: "Bug", Org: "acme", CompletedAt: at(yesterday, 9, 0)},
			{JiraKey: "A-2", IssueType: "Bug", Org: "acme", CompletedAt: at(yesterday, 10, 0)},
			{JiraKey: "A-3", IssueType: "Story", Org: "acme", CompletedAt: at(yesterday, 11, 0)},
			{JiraKey: "A-4", IssueType: "Task", Org: "", CompletedAt: at(yesterday, 12, 0)},
			{JiraKey: "A-5", IssueType: "", Org: "beta", CompletedAt: at(yesterday, 13, 0)},
		},
		TaskHours: 2,
	}

	got := Compute(in)

	require.Len(t, got.ByType, 4)
	assert.Equal(t, Slice{Label: "Bug", Count: 2}, got.ByType[0])
	// Story, Task, (none) all have count 1 -> alphabetical tie-break.
	assert.Equal(t, "(none)", got.ByType[1].Label)
	assert.Equal(t, 1, got.ByType[1].Count)
	assert.Equal(t, "Story", got.ByType[2].Label)
	assert.Equal(t, "Task", got.ByType[3].Label)

	require.Len(t, got.ByOrg, 3)
	assert.Equal(t, Slice{Label: "acme", Count: 3}, got.ByOrg[0])
	assert.Equal(t, "(none)", got.ByOrg[1].Label)
	assert.Equal(t, "beta", got.ByOrg[2].Label)
}

func TestComputeByTypeExcludesOutOfRangeCompletions(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)
	tooOld := now.AddDate(0, 0, -10)

	in := Input{
		Now:       now,
		RangeDays: 7,
		Completions: []Completion{
			{JiraKey: "A-1", IssueType: "Bug", Org: "acme", CompletedAt: at(tooOld, 9, 0)},
		},
		TaskHours: 2,
	}

	got := Compute(in)

	assert.Empty(t, got.ByType)
	assert.Empty(t, got.ByOrg)
}

func TestComputeAvgTaskMedianOdd(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	var completions []Completion
	var events []Event
	hoursAgo := []float64{1, 2, 3, 4, 100} // hours, odd count -> median is the middle value (3)
	for i, h := range hoursAgo {
		key := jiraKeyFor(i)
		start := now.Add(-time.Duration(h) * time.Hour)
		completions = append(completions, Completion{JiraKey: key, CompletedAt: now.Format(time.RFC3339)})
		events = append(events, Event{Kind: KindTaskStarted, JiraKey: key, OccurredAt: start.Format(time.RFC3339)})
	}

	got := Compute(Input{Now: now, Completions: completions, Events: events, TaskHours: 2})

	require.Equal(t, 5, got.AvgTask.Samples)
	assert.False(t, got.AvgTask.Assumed)
	assert.InDelta(t, 3.0, got.AvgTask.MedianHours, 0.001)
	assert.InDelta(t, 22.0, got.AvgTask.MeanHours, 0.001) // (1+2+3+4+100)/5
}

func TestComputeAvgTaskMedianEven(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	var completions []Completion
	var events []Event
	hoursAgo := []float64{1, 2, 3, 4, 5, 6} // even count -> median is avg of the two middles (3,4) = 3.5
	for i, h := range hoursAgo {
		key := jiraKeyFor(i)
		start := now.Add(-time.Duration(h) * time.Hour)
		completions = append(completions, Completion{JiraKey: key, CompletedAt: now.Format(time.RFC3339)})
		events = append(events, Event{Kind: KindTaskStarted, JiraKey: key, OccurredAt: start.Format(time.RFC3339)})
	}

	got := Compute(Input{Now: now, Completions: completions, Events: events, TaskHours: 2})

	require.Equal(t, 6, got.AvgTask.Samples)
	assert.False(t, got.AvgTask.Assumed)
	assert.InDelta(t, 3.5, got.AvgTask.MedianHours, 0.001)
}

func TestComputeAvgTask30DayGuard(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	completions := []Completion{
		{JiraKey: "OK", CompletedAt: now.Format(time.RFC3339)},
		{JiraKey: "TOO-LONG", CompletedAt: now.Format(time.RFC3339)},
		{JiraKey: "NEGATIVE", CompletedAt: now.Add(-time.Hour).Format(time.RFC3339)},
	}
	events := []Event{
		{Kind: KindTaskStarted, JiraKey: "OK", OccurredAt: now.Add(-5 * time.Hour).Format(time.RFC3339)},
		{Kind: KindTaskStarted, JiraKey: "TOO-LONG", OccurredAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339)},
		// started after it completed -> non-positive duration, must be dropped.
		{Kind: KindTaskStarted, JiraKey: "NEGATIVE", OccurredAt: now.Format(time.RFC3339)},
	}

	got := Compute(Input{Now: now, Completions: completions, Events: events, TaskHours: 2})

	require.Equal(t, 1, got.AvgTask.Samples)
	assert.True(t, got.AvgTask.Assumed, "still below the 5-sample floor")
}

func TestComputeAvgTaskAssumedFallback(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	completions := []Completion{
		{JiraKey: "A-1", CompletedAt: now.Format(time.RFC3339)},
		{JiraKey: "A-2", CompletedAt: now.Format(time.RFC3339)},
	}
	events := []Event{
		{Kind: KindTaskStarted, JiraKey: "A-1", OccurredAt: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{Kind: KindTaskStarted, JiraKey: "A-2", OccurredAt: now.Add(-6 * time.Hour).Format(time.RFC3339)},
	}

	got := Compute(Input{Now: now, Completions: completions, Events: events, TaskHours: 3.5})

	require.Equal(t, 2, got.AvgTask.Samples)
	assert.True(t, got.AvgTask.Assumed)
	assert.Equal(t, 3.5, got.AvgTask.MedianHours)
	assert.Equal(t, 3.5, got.AvgTask.MeanHours)
}

func TestComputeSkipsUnparseableTimestamps(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	in := Input{
		Now: now,
		Completions: []Completion{
			{JiraKey: "A-1", CompletedAt: "not-a-date"},
			{JiraKey: "A-2", CompletedAt: at(now, 9, 0)},
		},
		Events: []Event{
			{Kind: KindPRMerged, OccurredAt: "also-not-a-date"},
		},
		TaskHours: 2,
	}

	got := Compute(in)

	todayRow := got.Daily[len(got.Daily)-1]
	assert.Equal(t, 1, todayRow.Tasks)
	assert.Equal(t, 0, todayRow.PRs)
}

// jiraKeyFor generates distinct jira keys for table-driven duration tests.
func jiraKeyFor(i int) string {
	return string(rune('A' + i))
}
