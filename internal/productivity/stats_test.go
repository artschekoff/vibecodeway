package productivity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// at builds an RFC3339 timestamp for the given local day/hour, which is what
// Compute receives from the store.
func at(day time.Time, hour int) string {
	y, m, d := day.Date()
	return time.Date(y, m, d, hour, 0, 0, 0, time.Local).Format(time.RFC3339)
}

func TestComputeCountsTodayAndYesterday(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1)

	got := Compute(
		[]string{at(now, 9), at(now, 11), at(yesterday, 10)},
		now,
		Config{WorkdayHours: 8, TaskHours: 2},
	)

	assert.Equal(t, 2, got.Today)
	assert.Equal(t, 1, got.Yesterday)
	assert.Equal(t, 4.0, got.HoursToday)
}

func TestComputeWorkdayNorm(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	got := Compute([]string{at(now, 9), at(now, 11)}, now, Config{WorkdayHours: 8, TaskHours: 2})

	// 2 tasks × 2h = 4h of 8h → 2 more tasks to close the day.
	assert.Equal(t, 2, got.TasksToNorm)

	full := Compute(
		[]string{at(now, 9), at(now, 10), at(now, 11), at(now, 12), at(now, 13)},
		now,
		Config{WorkdayHours: 8, TaskHours: 2},
	)
	assert.Equal(t, 0, full.TasksToNorm, "norm already exceeded")
}

func TestComputeBaselinesAndCatchUp(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)
	var stamps []string
	// 7 tasks spread over the 7 days before today → week average 1.0/day.
	for i := 1; i <= 7; i++ {
		stamps = append(stamps, at(now.AddDate(0, 0, -i), 12))
	}
	// One task today.
	stamps = append(stamps, at(now, 9))

	got := Compute(stamps, now, Config{WorkdayHours: 8, TaskHours: 2})

	assert.Equal(t, 1, got.Today)
	assert.Equal(t, 1.0, got.WeekAvg)
	assert.InDelta(t, 7.0/30.0, got.MonthAvg, 0.0001)
	// Beating an average of 1.0 needs 2 today, so 1 more.
	assert.Equal(t, 1, got.TasksToBeatWeek)
	// Yesterday had 1 → 1 more needed to beat it.
	assert.Equal(t, 1, got.TasksToBeatYesterday)
	assert.Equal(t, 0, got.TasksToBeatMonth, "already above the month average")
}

func TestComputeDailySeriesEndsToday(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	got := Compute([]string{at(now, 9)}, now, Config{WorkdayHours: 8, TaskHours: 2})

	require.Len(t, got.Daily, DaysWindow)
	assert.Equal(t, "2026-08-17", got.Daily[DaysWindow-1].Day)
	assert.Equal(t, 1, got.Daily[DaysWindow-1].Count)
	assert.Equal(t, "2026-07-19", got.Daily[0].Day)
	assert.Equal(t, 0, got.Daily[0].Count)
}

func TestComputeIgnoresUnparseableAndZeroConfig(t *testing.T) {
	now := time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local)

	got := Compute([]string{"not-a-date", at(now, 9)}, now, Config{})

	assert.Equal(t, 1, got.Today)
	// Zero config falls back to 8h/2h so the dashboard never divides by zero.
	assert.Equal(t, 8.0, got.WorkdayHours)
	assert.Equal(t, 2.0, got.TaskHours)
	assert.Equal(t, 3, got.TasksToNorm)
}
