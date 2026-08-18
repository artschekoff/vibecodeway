// Package productivity turns raw task-completion timestamps into the numbers the
// productivity dashboard renders: per-day counts, baseline comparisons, and how
// much work is left to fill a theoretical workday. It is deliberately pure — no
// database, no clock — so every number is testable from a fixed `now`.
package productivity

import (
	"math"
	"time"
)

// DaysWindow is how many days the dashboard chart covers, including today.
const DaysWindow = 30

// Config is the user-tunable theoretical model (see app settings).
type Config struct {
	WorkdayHours float64 // length of a workday in hours, e.g. 8
	TaskHours    float64 // assumed cost of one closed task in hours, e.g. 2
}

// DayCount is the number of tasks closed on one local calendar day.
type DayCount struct {
	Day   string `json:"day"` // local YYYY-MM-DD
	Count int    `json:"count"`
}

// Stats is the whole dashboard payload.
type Stats struct {
	Today                int        `json:"today"`
	Yesterday            int        `json:"yesterday"`
	WeekAvg              float64    `json:"weekAvg"`  // per day over the 7 full days before today
	MonthAvg             float64    `json:"monthAvg"` // per day over the 30 full days before today
	Daily                []DayCount `json:"daily"`    // oldest → newest, DaysWindow entries ending today
	WorkdayHours         float64    `json:"workdayHours"`
	TaskHours            float64    `json:"taskHours"`
	HoursToday           float64    `json:"hoursToday"`
	TasksToNorm          int        `json:"tasksToNorm"`
	TasksToBeatYesterday int        `json:"tasksToBeatYesterday"`
	TasksToBeatWeek      int        `json:"tasksToBeatWeek"`
	TasksToBeatMonth     int        `json:"tasksToBeatMonth"`
}

// Compute buckets RFC3339 completion timestamps into local calendar days and
// derives the comparisons and the workday norm. Unparseable timestamps are
// skipped rather than failing the whole dashboard.
func Compute(completedAt []string, now time.Time, cfg Config) Stats {
	if cfg.WorkdayHours <= 0 {
		cfg.WorkdayHours = 8
	}
	if cfg.TaskHours <= 0 {
		cfg.TaskHours = 2
	}

	counts := make(map[string]int)
	for _, ts := range completedAt {
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		counts[dayKey(t.Local())]++
	}

	today := startOfDay(now)
	daily := make([]DayCount, 0, DaysWindow)
	for i := DaysWindow - 1; i >= 0; i-- {
		k := dayKey(today.AddDate(0, 0, -i))
		daily = append(daily, DayCount{Day: k, Count: counts[k]})
	}

	todayCount := counts[dayKey(today)]
	yesterday := counts[dayKey(today.AddDate(0, 0, -1))]
	weekAvg := avgPrevDays(counts, today, 7)
	monthAvg := avgPrevDays(counts, today, 30)

	hoursToday := float64(todayCount) * cfg.TaskHours
	tasksToNorm := 0
	if remaining := cfg.WorkdayHours - hoursToday; remaining > 0 {
		tasksToNorm = int(math.Ceil(remaining / cfg.TaskHours))
	}

	return Stats{
		Today:                todayCount,
		Yesterday:            yesterday,
		WeekAvg:              weekAvg,
		MonthAvg:             monthAvg,
		Daily:                daily,
		WorkdayHours:         cfg.WorkdayHours,
		TaskHours:            cfg.TaskHours,
		HoursToday:           hoursToday,
		TasksToNorm:          tasksToNorm,
		TasksToBeatYesterday: toBeat(float64(yesterday), todayCount),
		TasksToBeatWeek:      toBeat(weekAvg, todayCount),
		TasksToBeatMonth:     toBeat(monthAvg, todayCount),
	}
}

// toBeat is how many more tasks today needs to strictly beat the baseline.
// Fractional baselines round up to the next whole task (beating 3.4 needs 4).
func toBeat(baseline float64, today int) int {
	need := int(math.Floor(baseline)) + 1 - today
	if need < 0 {
		return 0
	}
	return need
}

// avgPrevDays averages the counts of the `days` full days before today; days
// with no closures count as zero, which is what "average per day" should mean.
func avgPrevDays(counts map[string]int, today time.Time, days int) float64 {
	total := 0
	for i := 1; i <= days; i++ {
		total += counts[dayKey(today.AddDate(0, 0, -i))]
	}
	return float64(total) / float64(days)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }
