package vcsstore_test

import (
	"context"
	"testing"
	"time"

	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewRequestHeadBranch(t *testing.T) {
	rr := vcsstore.ReviewRequest{HeadBranch: "feat/my-branch"}
	assert.Equal(t, "feat/my-branch", rr.HeadBranch)
}

func TestReviewRequestKindAndCIRoundTrip(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)

	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 7, "My PR", "me",
		"https://x/7", "changes_requested", "feat/x", "2026-08-01T00:00:00Z",
		`[{"login":"bob","state":"approved","submittedAt":""}]`, "MERGEABLE", "authored", "FAILURE", "")
	require.NoError(t, err)

	list, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	rr := list[0]
	if rr.Kind != "authored" || rr.CIStatus != "FAILURE" || len(rr.Reviewers) != 1 || rr.Reviewers[0].Login != "bob" {
		t.Fatalf("round-trip wrong: %+v", rr)
	}
}

func TestUpdateUnresolvedCountRoundTrips(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)

	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 7, "My PR", "me",
		"https://x/7", "review_required", "feat/x", "2026-08-01T00:00:00Z",
		"[]", "MERGEABLE", "requested", "", "")
	require.NoError(t, err)

	require.NoError(t, vcsStore.ReviewRequests.UpdateUnresolvedCount(ctx, repo.ID, 7, 3))
	list, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, 3, list[0].UnresolvedCount)

	// Resolved on the next sync → count drops back to 0.
	require.NoError(t, vcsStore.ReviewRequests.UpdateUnresolvedCount(ctx, repo.ID, 7, 0))
	list, err = vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, 0, list[0].UnresolvedCount)
}

func TestMarkReviewedOnlyTouchesRequested(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "octo", "hello2")
	require.NoError(t, err)

	_, _ = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 1, "req", "a", "u", "review_required", "b1", "", "[]", "MERGEABLE", "requested", "", "")
	_, _ = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 2, "mine", "me", "u", "changes_requested", "b2", "", "[]", "MERGEABLE", "authored", "SUCCESS", "")

	// No active requested PRs → requested #1 becomes reviewed; authored #2 must stay pending.
	require.NoError(t, vcsStore.ReviewRequests.MarkReviewedForRepo(ctx, repo.ID, nil))
	list, _ := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	byNum := map[int]vcsstore.ReviewRequest{}
	for _, rr := range list {
		byNum[rr.PRNumber] = rr
	}
	if byNum[1].ReviewedAt == nil {
		t.Fatal("requested PR #1 should be marked reviewed")
	}
	if byNum[2].ReviewedAt != nil {
		t.Fatal("authored PR #2 must NOT be marked reviewed")
	}
}

func TestUpsertAndListHeadBranch(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "testrepo")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 42,
		"My PR", "alice", "https://example.com/pr/42",
		"review_required", "feat/cool-thing", now, "", "", "requested", "", "")
	require.NoError(t, err)

	items, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "feat/cool-thing", items[0].HeadBranch)
}

func TestUpsertReviewRequest_InsertedFlag(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "insertedflagrepo")
	require.NoError(t, err)

	inserted, err := vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 42,
		"Title", "author", "https://example/pr/42", "changes_requested", "feat/x", "2026-07-20T00:00:00Z", "", "", "requested", "", "")
	require.NoError(t, err)
	assert.True(t, inserted, "first upsert should insert")

	inserted, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 42,
		"Title v2", "author", "https://example/pr/42", "approved", "feat/x", "2026-07-20T00:00:00Z", "", "", "requested", "", "")
	require.NoError(t, err)
	assert.False(t, inserted, "second upsert of same (repo,pr) should update, not insert")
}

func TestReviewRequests_ViewedAndReviewers(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "api")
	require.NoError(t, err)

	reviewers := `[{"login":"alice","state":"approved","submittedAt":"2026-07-20T10:00:00Z"},{"login":"bob","state":"changes_requested","submittedAt":"2026-07-21T10:00:00Z"}]`
	inserted, err := vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 368, "Title", "carol", "https://x/368", "review_required", "feat/x", "2026-07-19T10:00:00Z", reviewers, "CONFLICTING", "requested", "", "")
	require.NoError(t, err)
	require.True(t, inserted)

	list, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	rr := list[0]
	require.Nil(t, rr.ViewedAt) // new = unviewed
	require.Len(t, rr.Reviewers, 2)
	require.Equal(t, "alice", rr.Reviewers[0].Login)
	require.Equal(t, "approved", rr.Reviewers[0].State)
	require.Equal(t, "changes_requested", rr.Reviewers[1].State)
	require.Equal(t, "CONFLICTING", rr.Mergeable) // mergeable round-trips through the DB

	// Mark viewed.
	require.NoError(t, vcsStore.ReviewRequests.MarkReviewRequestViewed(ctx, rr.ID))
	list, _ = vcsStore.ReviewRequests.ListReviewRequests(ctx, "")
	require.NotNil(t, list[0].ViewedAt)

	// Re-sync (UPSERT update branch) preserves viewed_at, refreshes reviewers + mergeable.
	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 368, "Title2", "carol", "https://x/368", "changes_requested", "feat/x", "2026-07-19T10:00:00Z", `[{"login":"bob","state":"changes_requested","submittedAt":"2026-07-22T10:00:00Z"}]`, "MERGEABLE", "requested", "", "")
	require.NoError(t, err)
	list, _ = vcsStore.ReviewRequests.ListReviewRequests(ctx, "")
	require.NotNil(t, list[0].ViewedAt)              // preserved
	require.Len(t, list[0].Reviewers, 1)             // refreshed
	require.Equal(t, "Title2", list[0].PRTitle)      // updated
	require.Equal(t, "MERGEABLE", list[0].Mergeable) // refreshed on update

	// Mark all viewed clears any remaining unviewed.
	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 369, "T", "d", "u", "review_required", "b", "t", "", "", "requested", "", "")
	require.NoError(t, err)
	require.NoError(t, vcsStore.ReviewRequests.MarkAllReviewRequestsViewed(ctx))
	list, _ = vcsStore.ReviewRequests.ListReviewRequests(ctx, "")
	for _, r := range list {
		require.NotNil(t, r.ViewedAt)
	}
}

func TestSetReviewStatusPreservesReviewedAt(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "mergerepo")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 7,
		"Merged PR", "alice", "https://example.com/pr/7",
		"approved", "feat/done", now, "", "", "requested", "", "")
	require.NoError(t, err)

	// Simulate the PR dropping off the open lists → marked reviewed (waiting).
	require.NoError(t, vcsStore.ReviewRequests.MarkReviewedForRepo(ctx, repo.ID, nil))

	// Relabel it as merged — must NOT clear reviewed_at.
	require.NoError(t, vcsStore.ReviewRequests.SetReviewStatus(ctx, repo.ID, 7, "merged"))

	items, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "merged", items[0].ReviewStatus)
	require.NotNil(t, items[0].ReviewedAt) // still "waiting" (reviewed) — only the label changed
}

func TestUpdateMyReviewStateRoundTrips(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)

	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 11, "PR", "carol",
		"https://x/11", "changes_requested", "feat/x", "2026-08-01T00:00:00Z",
		"[]", "MERGEABLE", "requested", "SUCCESS", "")
	require.NoError(t, err)

	// A fresh row has no standing decision.
	list, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "", list[0].MyReviewState)

	require.NoError(t, vcsStore.ReviewRequests.UpdateMyReviewState(ctx, repo.ID, 11, "changes_requested"))

	list, err = vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "changes_requested", list[0].MyReviewState)
}

func TestUpsertKeepsMyReviewState(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "octo", "hello")
	require.NoError(t, err)

	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 12, "PR", "carol",
		"https://x/12", "review_required", "feat/y", "2026-08-01T00:00:00Z",
		"[]", "MERGEABLE", "requested", "", "")
	require.NoError(t, err)
	require.NoError(t, vcsStore.ReviewRequests.UpdateMyReviewState(ctx, repo.ID, 12, "approved"))

	// Re-upsert (the PR re-appeared in a GitHub search) must not wipe the decision.
	_, err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 12, "PR", "carol",
		"https://x/12", "review_required", "feat/y", "2026-08-01T00:00:00Z",
		"[]", "MERGEABLE", "requested", "", "")
	require.NoError(t, err)

	list, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "approved", list[0].MyReviewState)
}
