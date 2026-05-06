package vcsstore_test

import (
	"context"
	"testing"
	"time"

	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewRequestHeadBranch(t *testing.T) {
	rr := vcsstore.ReviewRequest{HeadBranch: "feat/my-branch"}
	assert.Equal(t, "feat/my-branch", rr.HeadBranch)
}

func TestUpsertAndListHeadBranch(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "testrepo")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)

	err = vcsStore.ReviewRequests.UpsertReviewRequest(ctx, repo.ID, 42,
		"My PR", "alice", "https://example.com/pr/42",
		"review_required", "feat/cool-thing", now)
	require.NoError(t, err)

	items, err := vcsStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "feat/cool-thing", items[0].HeadBranch)
}
