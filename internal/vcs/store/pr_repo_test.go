package vcsstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) (*appstore.Store, *vcsstore.Store) {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s, vcsstore.NewStore(s.DB())
}

func TestUpsertPR_RoundTripsMergeStateStatus(t *testing.T) {
	s, vcsStore := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "web")
	require.NoError(t, err)
	branch, err := vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "feat/x", "")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, 7,
		"t", "a", "open", "review_required", "http://u", "feat/x",
		"MERGEABLE", "BEHIND", "SUCCESS", false, now, now)
	require.NoError(t, err)

	got, err := vcsStore.PullRequests.GetPRByRepoAndNumber(ctx, repo.ID, 7)
	require.NoError(t, err)
	assert.Equal(t, "BEHIND", got.MergeStateStatus)
}

func TestListCommentsForPRs(t *testing.T) {
	s, vcsStore := openTestDB(t)
	ctx := context.Background()

	// Create a context
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	// Create a VCS repo
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "repo")
	require.NoError(t, err)

	// Create a branch
	branch, err := vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "main", "")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)

	// Create two PRs
	pr1Result, err := vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, 1,
		"PR 1", "alice", "open", "approved", "https://example.com/pr/1",
		"feature-1", "mergeable", "", "success", false, now, now)
	require.NoError(t, err)
	pr1ID := pr1Result.ID

	pr2Result, err := vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, 2,
		"PR 2", "bob", "open", "pending", "https://example.com/pr/2",
		"feature-2", "mergeable", "", "success", false, now, now)
	require.NoError(t, err)
	pr2ID := pr2Result.ID

	// Insert 1 comment on PR1
	_, _, err = vcsStore.PullRequests.UpsertComment(ctx, pr1ID, "ext-1", nil,
		"review", "alice", "LGTM", nil, nil, false, nil, now, now)
	require.NoError(t, err)

	// Insert 2 comments on PR2
	_, _, err = vcsStore.PullRequests.UpsertComment(ctx, pr2ID, "ext-2", nil,
		"review", "bob", "Needs work", nil, nil, false, nil, now, now)
	require.NoError(t, err)

	_, _, err = vcsStore.PullRequests.UpsertComment(ctx, pr2ID, "ext-3", nil,
		"review", "carol", "Agreed", nil, nil, false, nil, now, now)
	require.NoError(t, err)

	// Call ListCommentsForPRs with both PR IDs
	got, err := vcsStore.PullRequests.ListCommentsForPRs(ctx, []string{pr1ID, pr2ID})
	require.NoError(t, err)

	assert.Len(t, got[pr1ID], 1, "pr1 should have 1 comment")
	assert.Len(t, got[pr2ID], 2, "pr2 should have 2 comments")
	assert.Equal(t, "LGTM", got[pr1ID][0].Body)

	// Nil input returns empty map without error
	empty, err := vcsStore.PullRequests.ListCommentsForPRs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestUpsertCommentReportsIsNew(t *testing.T) {
	s, vcsStore := openTestDB(t)
	ctx := context.Background()

	// Create a context
	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)

	// Create a VCS repo
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "repo")
	require.NoError(t, err)

	// Create a branch
	branch, err := vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "main", "")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)

	prResult, err := vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, 1,
		"PR 1", "alice", "open", "approved", "https://example.com/pr/1",
		"feature-1", "mergeable", "", "success", false, now, now)
	require.NoError(t, err)
	prID := prResult.ID

	// First upsert of (prID, externalID) inserts a brand-new comment.
	_, isNew, err := vcsStore.PullRequests.UpsertComment(ctx, prID, "ext-new", nil,
		"review", "alice", "nit", nil, nil, false, nil,
		"2024-01-01T00:00:00Z", "2024-01-01T00:00:00Z")
	require.NoError(t, err)
	assert.True(t, isNew, "first upsert of a new (prID, externalID) should report isNew=true")

	// Second upsert of the SAME (prID, externalID) updates the existing comment.
	_, isNew, err = vcsStore.PullRequests.UpsertComment(ctx, prID, "ext-new", nil,
		"review", "alice", "nit edited", nil, nil, false, nil,
		"2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z")
	require.NoError(t, err)
	assert.False(t, isNew, "second upsert of an existing (prID, externalID) should report isNew=false")
}

// Reconciling stale board PRs: an open PR whose remote state is now "merged" must be
// updatable so the board (which reads state='open') stops showing it.
func TestListOpenPRsWithRepoAndSetPRState(t *testing.T) {
	s, vcsStore := openTestDB(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "test-ctx", org.ID)
	require.NoError(t, err)
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "api")
	require.NoError(t, err)
	branch, err := vcsStore.Branches.AddBranch(ctx, c.ID, repo.ID, "feat/x", "")
	require.NoError(t, err)

	now := time.Now().UTC().Format(time.RFC3339)
	for _, n := range []int{1, 2} {
		_, err = vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, n,
			"PR", "alice", "open", "", "url", "feat/x", "mergeable", "", "success", false, now, now)
		require.NoError(t, err)
	}

	open, err := vcsStore.PullRequests.ListOpenPRsWithRepo(ctx)
	require.NoError(t, err)
	require.Len(t, open, 2)
	assert.Equal(t, repo.ID, open[0].RepoID)
	assert.Equal(t, "github", open[0].Platform)
	assert.Equal(t, "acme", open[0].Owner)
	assert.Equal(t, "api", open[0].Name)

	// PR #1 merged on the remote → reconcile it.
	require.NoError(t, vcsStore.PullRequests.SetPRState(ctx, repo.ID, 1, "merged"))

	got, err := vcsStore.PullRequests.GetPRByRepoAndNumber(ctx, repo.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "merged", got.State)

	// Only the still-open PR #2 remains in the open list.
	open, err = vcsStore.PullRequests.ListOpenPRsWithRepo(ctx)
	require.NoError(t, err)
	require.Len(t, open, 1)
	assert.Equal(t, 2, open[0].PRNumber)
}
