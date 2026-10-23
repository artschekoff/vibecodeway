package github

import (
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	"github.com/stretchr/testify/assert"
)

// After the author re-requests a review, the reviewer whose "changes
// requested" still stands on GitHub is shown as pending and the PR is no
// longer "changes requested" — the next move is the reviewer's.
func TestWithPendingReviewersReRequest(t *testing.T) {
	pr := vcs.PRData{
		ReviewStatus: "changes_requested",
		Reviewers: []vcs.ReviewerDecision{
			{Login: "bot", State: "commented"},
			{Login: "dan", State: "changes_requested"},
		},
		PendingReviewers: []string{"dan", "mukh"},
	}
	got := withPendingReviewers(pr)
	assert.Equal(t, "review_required", got.ReviewStatus)
	assert.Equal(t, []vcs.ReviewerDecision{
		{Login: "bot", State: "commented"},
		{Login: "dan", State: "pending"},
		{Login: "mukh", State: "pending"},
	}, got.Reviewers)

	// A second blocker who was NOT re-requested keeps the PR blocked.
	pr.Reviewers = append(pr.Reviewers, vcs.ReviewerDecision{Login: "eve", State: "changes_requested"})
	assert.Equal(t, "changes_requested", withPendingReviewers(pr).ReviewStatus)
	// No pending requests: untouched.
	assert.Equal(t, "changes_requested", withPendingReviewers(vcs.PRData{ReviewStatus: "changes_requested"}).ReviewStatus)
}
