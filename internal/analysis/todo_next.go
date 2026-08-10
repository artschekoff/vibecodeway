package analysis

import (
	"time"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// TodoInput is the board-wide snapshot handed to the AI for TODO generation.
type TodoInput struct {
	JiraIssues     []appstore.BoardJiraAssignment
	MyPRs          []appstore.BoardPR
	ReviewRequests []vcsstore.ReviewRequest
	StalePRNumbers []int
	ThresholdDays  int
	Now            time.Time
}

// isPRStale reports whether a PR last updated at updatedAt is older than
// thresholdDays. Empty or unparseable timestamps are never stale.
func isPRStale(updatedAt string, now time.Time, thresholdDays int) bool {
	if updatedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return false
	}
	return now.Sub(t) >= time.Duration(thresholdDays)*24*time.Hour
}

// computeStalePRs returns the numbers of open PRs that are stale and still need
// reviewer action (not approved/merged). These become rerequest_pr todos.
func computeStalePRs(prs []appstore.BoardPR, now time.Time, thresholdDays int) []int {
	var out []int
	for _, pr := range prs {
		if pr.State != "open" {
			continue
		}
		if pr.ReviewStatus == "approved" {
			continue
		}
		if isPRStale(pr.PRUpdatedAt, now, thresholdDays) {
			out = append(out, pr.PRNumber)
		}
	}
	return out
}
