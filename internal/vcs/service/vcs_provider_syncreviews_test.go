package vcsservice

import (
	"context"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SyncReviews must walk every distinct repo exactly once (case-duplicates
// collapsed), store both requested and authored rows in that single pass, and
// refuse to run while another sync holds the lock.
func TestSyncReviewsSinglePassDedupedAndGuarded(t *testing.T) {
	runner := &mockRunner{
		platform:       vcs.PlatformGitHub,
		reviewRequests: []vcs.PRData{{Number: 1, Title: "theirs", Author: "alice", State: "open", URL: "u1", HeadBranch: "a"}},
		authored:       []vcs.PRData{{Number: 2, Title: "mine", Author: "me", State: "open", URL: "u2", HeadBranch: "b"}},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)

	var progress []string
	p.WithReviewProgressFunc(func(current, total int, repo, phase string) {
		progress = append(progress, phase+":"+repo)
		assert.Equal(t, 1, total)
	})

	require.NoError(t, p.SyncReviews(ctx, vcs.PlatformGitHub))
	assert.Equal(t, []string{"sync:octo/hello"}, progress, "one progress tick per distinct repo, one phase")

	rows, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	kinds := map[int]string{}
	for _, r := range rows {
		kinds[r.PRNumber] = r.Kind
	}
	assert.Equal(t, map[int]string{1: "requested", 2: "authored"}, kinds)

	p.reviewSyncMu.Lock()
	assert.ErrorIs(t, p.SyncReviews(ctx, vcs.PlatformGitHub), ErrReviewSyncRunning)
	p.reviewSyncMu.Unlock()
}

func TestDedupeReposCollapsesCaseVariants(t *testing.T) {
	repos := []vcsstore.VCSRepo{
		{ID: "a", Platform: "github", Owner: "octo", Name: "hello"},
		{ID: "b", Platform: "github", Owner: "Octo", Name: "Hello"}, // re-added with different case
		{ID: "c", Platform: "github", Owner: "octo", Name: "other"},
	}
	got := dedupeRepos(repos)
	ids := []string{}
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	assert.Equal(t, []string{"a", "c"}, ids, "first occurrence wins, distinct repos kept")
}

// `reviewed-by:@me review:changes_requested` also matches PRs I merely
// commented on while someone else blocked them, and my own PRs. Neither is
// "my changes requested".
func TestSyncReviewsDoesNotMisTagMyReviewState(t *testing.T) {
	runner := &mockRunner{
		platform: vcs.PlatformGitHub,
		changesRequested: []vcs.PRData{
			{Number: 1, Title: "theirs, blocked by dan", Author: "alice", State: "open", ReviewStatus: "changes_requested", URL: "u1", HeadBranch: "a",
				Reviewers: []vcs.ReviewerDecision{{Login: "me", State: "commented"}, {Login: "dan", State: "changes_requested"}}},
			{Number: 2, Title: "theirs, blocked by me", Author: "alice", State: "open", ReviewStatus: "changes_requested", URL: "u2", HeadBranch: "b",
				Reviewers: []vcs.ReviewerDecision{{Login: "me", State: "changes_requested"}}},
			{Number: 3, Title: "mine", Author: "me", State: "open", ReviewStatus: "changes_requested", URL: "u3", HeadBranch: "c"},
		},
	}
	p, vs := newTestProvider(t, runner)
	ctx := context.Background()
	repo, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)
	require.NoError(t, p.SyncReviews(ctx, vcs.PlatformGitHub))

	rows, err := vs.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	state := map[int]string{}
	for _, r := range rows {
		state[r.PRNumber] = r.MyReviewState
	}
	assert.Equal(t, "", state[1], "a comment on a PR dan blocked is not my decision")
	assert.Equal(t, "changes_requested", state[2])
	_, mineStored := state[3]
	assert.False(t, mineStored, "my own PR never enters the requested list")
}
