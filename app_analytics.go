package main

import (
	"fmt"
	"time"

	"github.com/artschekoff/vibecodeway/internal/analytics"
)

// GetAnalyticsDashboard returns the analytics dashboard payload for the
// requested range: period-over-period cards, a daily series, breakdowns by
// issue type and org, and average task duration.
func (a *App) GetAnalyticsDashboard(rangeDays int) (*analytics.Dashboard, error) {
	// Clamp before computing since, not after: Compute defaults an
	// out-of-range RangeDays internally, but by then the fetch window below
	// would already be wrong (e.g. rangeDays<=0 fetches ~1 day of rows while
	// Compute still renders a 30-day Daily series, or negative rangeDays
	// pushes since into the future and returns nothing). Same pattern as
	// app_activity.go's limit<=0 clamp.
	if rangeDays <= 0 {
		rangeDays = analytics.DefaultRangeDays
	}

	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("analytics dashboard: %w", err)
	}
	now := time.Now()
	// Cards compare a trailing period against the preceding period of the
	// same length, so we need 2*rangeDays of history, plus one extra day of
	// slack so completions/events near a local-midnight boundary aren't cut
	// off by the UTC comparison in SQL (same reasoning as app_productivity.go).
	since := now.AddDate(0, 0, -(2*rangeDays + 1)).UTC().Format(time.RFC3339)

	completionRows, err := a.db.ListTaskCompletionsSince(a.ctx, since)
	if err != nil {
		return nil, fmt.Errorf("analytics dashboard: %w", err)
	}
	eventRows, err := a.db.ListStatEventsSince(a.ctx, since)
	if err != nil {
		return nil, fmt.Errorf("analytics dashboard: %w", err)
	}

	completions := make([]analytics.Completion, 0, len(completionRows))
	for _, r := range completionRows {
		completions = append(completions, analytics.Completion{
			JiraKey:     r.Key,
			IssueType:   r.IssueType,
			Org:         r.Org,
			CompletedAt: r.CompletedAt,
		})
	}
	events := make([]analytics.Event, 0, len(eventRows))
	for _, r := range eventRows {
		events = append(events, analytics.Event{
			Kind:       r.Kind,
			ExternalID: r.ExternalID,
			JiraKey:    r.JiraKey,
			OccurredAt: r.OccurredAt,
		})
	}

	dashboard := analytics.Compute(analytics.Input{
		Completions: completions,
		Events:      events,
		Now:         now,
		RangeDays:   rangeDays,
		TaskHours:   settings.TaskHours,
	})
	return &dashboard, nil
}
