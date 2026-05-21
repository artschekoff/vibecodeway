package vcsstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
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
		"feature-1", "mergeable", "success", now, now)
	require.NoError(t, err)
	pr1ID := pr1Result.ID

	pr2Result, err := vcsStore.PullRequests.UpsertPR(ctx, repo.ID, branch.ID, 2,
		"PR 2", "bob", "open", "pending", "https://example.com/pr/2",
		"feature-2", "mergeable", "success", now, now)
	require.NoError(t, err)
	pr2ID := pr2Result.ID

	// Insert 1 comment on PR1
	_, err = vcsStore.PullRequests.UpsertComment(ctx, pr1ID, "ext-1", nil,
		"review", "alice", "LGTM", nil, nil, false, nil, now, now)
	require.NoError(t, err)

	// Insert 2 comments on PR2
	_, err = vcsStore.PullRequests.UpsertComment(ctx, pr2ID, "ext-2", nil,
		"review", "bob", "Needs work", nil, nil, false, nil, now, now)
	require.NoError(t, err)

	_, err = vcsStore.PullRequests.UpsertComment(ctx, pr2ID, "ext-3", nil,
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
