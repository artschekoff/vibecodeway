// Package analytics turns raw completion and event rows into the numbers the
// analytics dashboard renders: period-over-period cards, a daily series,
// breakdowns by issue type and org, and average task duration. It is
// deliberately pure — no database, no clock — so every number is testable
// from a fixed `now`, following the same conventions as
// internal/productivity.
package analytics

import (
	"sort"
	"time"
)

// DefaultRangeDays is used when Input.RangeDays is not one of the supported
// values.
const DefaultRangeDays = 30

// Event kinds recognized in Input.Events.
const (
	KindPRMerged        = "pr_merged"
	KindCommentAuthored = "comment_authored"
	KindTaskStarted     = "task_started"
)

// Metric names used on PeriodCard.
const (
	MetricTasksClosed = "tasksClosed"
	MetricPRsMerged   = "prsMerged"
	MetricComments    = "comments"
	MetricAITokens    = "aiTokens"
)

// Period names used on PeriodCard.
const (
	PeriodYesterday = "yesterday"
	PeriodWeek      = "week"
	PeriodMonth     = "month"
)

// noneLabel is used for Slice entries whose grouping key is empty.
const noneLabel = "(none)"

// maxTaskDuration is the upper bound (exclusive) on a valid task duration
// used for AvgTask; pairs at or beyond it are treated as noise (e.g. a
// mis-tagged jira_key reused months apart) and dropped.
const maxTaskDuration = 30 * 24 * time.Hour

// minAvgTaskSamples is the minimum number of valid duration samples before
// AvgTask reports real numbers instead of the configured fallback.
const minAvgTaskSamples = 5

// Completion is one closed task/issue.
type Completion struct {
	JiraKey     string
	IssueType   string
	Org         string
	CompletedAt string // RFC3339
}

// Event is one tracked activity row (a PR merge, a comment, or a task
// start).
type Event struct {
	Kind       string
	ExternalID string
	JiraKey    string
	OccurredAt string // RFC3339
}

// TokenUsage is one LLM call's token bill (an ai_usage store row).
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	OccurredAt       string // RFC3339
}

// Input is everything Compute needs. Completions/Events are store rows the
// binding fetches; this package never touches the store itself.
type Input struct {
	Completions []Completion
	Events      []Event
	AIUsage     []TokenUsage
	Now         time.Time // local tz drives day bucketing, like productivity.Compute
	RangeDays   int       // 7|30|90; <=0 defaults to DefaultRangeDays
	TaskHours   float64   // fallback assumed hours per task (settings)
}

// Dashboard is the whole analytics dashboard payload.
type Dashboard struct {
	RangeDays int          `json:"rangeDays"`
	Cards     []PeriodCard `json:"cards"`
	Daily     []DayRow     `json:"daily"`
	ByType    []Slice      `json:"byType"`
	ByOrg     []Slice      `json:"byOrg"`
	AvgTask   AvgTask      `json:"avgTask"`
	AiTokens  AITokens     `json:"aiTokens"`
}

// AITokens sums the LLM token bill over the selected range.
type AITokens struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
	Calls            int `json:"calls"`
}

// PeriodCard compares one metric over one trailing period against the
// same-length preceding period.
type PeriodCard struct {
	Metric   string  `json:"metric"` // tasksClosed | prsMerged | comments
	Period   string  `json:"period"` // yesterday | week | month
	Count    int     `json:"count"`
	Prev     int     `json:"prev"`     // same-length preceding period
	DeltaPct float64 `json:"deltaPct"` // 0 when prev==0 && count==0; prev==0&&count>0 => 100
}

// DayRow is one local calendar day's counts in the Daily series.
type DayRow struct {
	Day      string `json:"day"` // local YYYY-MM-DD
	Tasks    int    `json:"tasks"`
	PRs      int    `json:"prs"`
	Comments int    `json:"comments"`
}

// Slice is one bucket of a breakdown (by issue type or by org).
type Slice struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// AvgTask is the median/mean time to close a task, computed from
// task_started → Completion pairs joined on jira_key.
type AvgTask struct {
	MedianHours float64 `json:"medianHours"`
	MeanHours   float64 `json:"meanHours"`
	Samples     int     `json:"samples"`
	Assumed     bool    `json:"assumed"` // true when Samples < 5 -> Median/Mean = TaskHours fallback
}

// parsedCompletion is a Completion with its timestamp parsed and localized.
type parsedCompletion struct {
	jiraKey     string
	issueType   string
	org         string
	completedAt time.Time
}

// parsedEvent is an Event with its timestamp parsed and localized.
type parsedEvent struct {
	kind       string
	jiraKey    string
	occurredAt time.Time
}

// Compute buckets RFC3339 completion/event timestamps into local calendar
// days and derives the period cards, daily series, breakdowns, and average
// task duration. Unparseable timestamps are skipped rather than failing the
// whole dashboard.
func Compute(in Input) Dashboard {
	rangeDays := in.RangeDays
	if rangeDays <= 0 {
		rangeDays = DefaultRangeDays
	}

	completions := parseCompletions(in.Completions)
	events := parseEvents(in.Events)

	tasksByDay := make(map[string]int)
	for _, c := range completions {
		tasksByDay[dayKey(c.completedAt)]++
	}
	prsByDay := make(map[string]int)
	commentsByDay := make(map[string]int)
	for _, e := range events {
		switch e.kind {
		case KindPRMerged:
			prsByDay[dayKey(e.occurredAt)]++
		case KindCommentAuthored:
			commentsByDay[dayKey(e.occurredAt)]++
		}
	}

	today := startOfDay(in.Now)

	// LLM token usage: daily buckets feed the cards; the range window feeds the
	// aiTokens summary tile. Unparseable timestamps are skipped, as elsewhere.
	tokensByDay := make(map[string]int)
	var aiTotals AITokens
	rangeStart := today.AddDate(0, 0, -(rangeDays - 1))
	for _, u := range in.AIUsage {
		t, err := time.Parse(time.RFC3339, u.OccurredAt)
		if err != nil {
			continue
		}
		local := t.Local()
		tokensByDay[dayKey(local)] += u.TotalTokens
		if !local.Before(rangeStart) {
			aiTotals.PromptTokens += u.PromptTokens
			aiTotals.CompletionTokens += u.CompletionTokens
			aiTotals.TotalTokens += u.TotalTokens
			aiTotals.Calls++
		}
	}

	cards := buildCards(today, tasksByDay, prsByDay, commentsByDay, tokensByDay)
	daily := buildDaily(today, rangeDays, tasksByDay, prsByDay, commentsByDay)
	byType, byOrg := buildBreakdowns(completions, today, rangeDays)
	avgTask := buildAvgTask(completions, events, in.TaskHours)

	return Dashboard{
		RangeDays: rangeDays,
		Cards:     cards,
		Daily:     daily,
		ByType:    byType,
		ByOrg:     byOrg,
		AvgTask:   avgTask,
		AiTokens:  aiTotals,
	}
}

func parseCompletions(in []Completion) []parsedCompletion {
	out := make([]parsedCompletion, 0, len(in))
	for _, c := range in {
		t, err := time.Parse(time.RFC3339, c.CompletedAt)
		if err != nil {
			continue
		}
		out = append(out, parsedCompletion{
			jiraKey:     c.JiraKey,
			issueType:   c.IssueType,
			org:         c.Org,
			completedAt: t.Local(),
		})
	}
	return out
}

func parseEvents(in []Event) []parsedEvent {
	out := make([]parsedEvent, 0, len(in))
	for _, e := range in {
		t, err := time.Parse(time.RFC3339, e.OccurredAt)
		if err != nil {
			continue
		}
		out = append(out, parsedEvent{
			kind:       e.Kind,
			jiraKey:    e.JiraKey,
			occurredAt: t.Local(),
		})
	}
	return out
}

// buildCards produces the 12 period cards, ordered metric-major: tasksClosed
// y/w/m, prsMerged y/w/m, comments y/w/m, aiTokens y/w/m. Today is excluded —
// it's incomplete.
func buildCards(today time.Time, tasksByDay, prsByDay, commentsByDay, tokensByDay map[string]int) []PeriodCard {
	type metric struct {
		name   string
		counts map[string]int
	}
	metrics := []metric{
		{MetricTasksClosed, tasksByDay},
		{MetricPRsMerged, prsByDay},
		{MetricComments, commentsByDay},
		{MetricAITokens, tokensByDay},
	}
	type period struct {
		name             string
		startAgo, length int
	}
	periods := []period{
		{PeriodYesterday, 1, 1},
		{PeriodWeek, 1, 7},
		{PeriodMonth, 1, 30},
	}

	cards := make([]PeriodCard, 0, len(metrics)*len(periods))
	for _, m := range metrics {
		for _, p := range periods {
			count := sumDays(m.counts, today, p.startAgo, p.length)
			prev := sumDays(m.counts, today, p.startAgo+p.length, p.length)
			cards = append(cards, PeriodCard{
				Metric:   m.name,
				Period:   p.name,
				Count:    count,
				Prev:     prev,
				DeltaPct: deltaPct(count, prev),
			})
		}
	}
	return cards
}

// deltaPct is the percent change from prev to count. 0 when both are 0;
// exactly 100 when prev is 0 but count is positive (can't divide by zero,
// but growth-from-nothing is treated as a full swing).
func deltaPct(count, prev int) float64 {
	if prev == 0 {
		if count == 0 {
			return 0
		}
		return 100
	}
	return (float64(count) - float64(prev)) / float64(prev) * 100
}

// sumDays sums `days` counts starting `startAgo` days before today and
// going further back, e.g. sumDays(c, today, 1, 7) sums today-1..today-7.
func sumDays(counts map[string]int, today time.Time, startAgo, days int) int {
	total := 0
	for i := startAgo; i < startAgo+days; i++ {
		total += counts[dayKey(today.AddDate(0, 0, -i))]
	}
	return total
}

// buildDaily produces rangeDays entries ending today, oldest -> newest.
func buildDaily(today time.Time, rangeDays int, tasksByDay, prsByDay, commentsByDay map[string]int) []DayRow {
	daily := make([]DayRow, 0, rangeDays)
	for i := rangeDays - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		k := dayKey(day)
		daily = append(daily, DayRow{
			Day:      k,
			Tasks:    tasksByDay[k],
			PRs:      prsByDay[k],
			Comments: commentsByDay[k],
		})
	}
	return daily
}

// buildBreakdowns groups completions in the selected range (rangeDays
// entries ending today, same window as Daily) by issue type and by org.
func buildBreakdowns(completions []parsedCompletion, today time.Time, rangeDays int) (byType, byOrg []Slice) {
	rangeStart := today.AddDate(0, 0, -(rangeDays - 1))
	typeCounts := make(map[string]int)
	orgCounts := make(map[string]int)
	for _, c := range completions {
		day := startOfDay(c.completedAt)
		if day.Before(rangeStart) || day.After(today) {
			continue
		}
		typeCounts[c.issueType]++
		orgCounts[c.org]++
	}
	return sliceFromCounts(typeCounts), sliceFromCounts(orgCounts)
}

// sliceFromCounts turns a label->count map into Slices sorted desc by
// count, then asc by label for a stable order among ties. An empty label
// becomes "(none)".
func sliceFromCounts(counts map[string]int) []Slice {
	out := make([]Slice, 0, len(counts))
	for label, count := range counts {
		if label == "" {
			label = noneLabel
		}
		out = append(out, Slice{Label: label, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// buildAvgTask joins task_started events to completions on jira_key and
// derives the median/mean duration. When a jira_key has more than one
// task_started event, the earliest is used as the start.
func buildAvgTask(completions []parsedCompletion, events []parsedEvent, taskHours float64) AvgTask {
	starts := make(map[string]time.Time)
	for _, e := range events {
		if e.kind != KindTaskStarted {
			continue
		}
		if existing, ok := starts[e.jiraKey]; !ok || e.occurredAt.Before(existing) {
			starts[e.jiraKey] = e.occurredAt
		}
	}

	var hours []float64
	for _, c := range completions {
		start, ok := starts[c.jiraKey]
		if !ok {
			continue
		}
		duration := c.completedAt.Sub(start)
		if duration <= 0 || duration >= maxTaskDuration {
			continue
		}
		hours = append(hours, duration.Hours())
	}

	samples := len(hours)
	if samples < minAvgTaskSamples {
		return AvgTask{
			MedianHours: taskHours,
			MeanHours:   taskHours,
			Samples:     samples,
			Assumed:     true,
		}
	}

	sort.Float64s(hours)
	sum := 0.0
	for _, h := range hours {
		sum += h
	}
	mean := sum / float64(samples)

	var median float64
	mid := samples / 2
	if samples%2 == 0 {
		median = (hours[mid-1] + hours[mid]) / 2
	} else {
		median = hours[mid]
	}

	return AvgTask{
		MedianHours: median,
		MeanHours:   mean,
		Samples:     samples,
		Assumed:     false,
	}
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }
