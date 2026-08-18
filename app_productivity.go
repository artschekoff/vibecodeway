package main

import (
	"fmt"
	"time"

	"github.com/artschekoff/vibecodeway/internal/productivity"
)

// GetProductivityStats returns the closed-task dashboard payload: per-day counts
// for the chart window, comparisons against yesterday / last week / last month,
// and how many tasks are still missing to fill the theoretical workday.
func (a *App) GetProductivityStats() (*productivity.Stats, error) {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("productivity stats: %w", err)
	}
	now := time.Now()
	// One extra day of slack so completions near a local-midnight boundary are
	// not cut off by the UTC comparison in SQL.
	since := now.AddDate(0, 0, -(productivity.DaysWindow + 1)).UTC().Format(time.RFC3339)
	rows, err := a.db.ListTaskCompletionsSince(a.ctx, since)
	if err != nil {
		return nil, fmt.Errorf("productivity stats: %w", err)
	}
	stamps := make([]string, 0, len(rows))
	for _, r := range rows {
		stamps = append(stamps, r.CompletedAt)
	}
	stats := productivity.Compute(stamps, now, productivity.Config{
		WorkdayHours: settings.WorkdayHours,
		TaskHours:    settings.TaskHours,
	})
	return &stats, nil
}
