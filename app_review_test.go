package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReviewTestApp(t *testing.T) *App {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	vs := vcsstore.NewStore(s.DB())
	return &App{db: s, vcsStore: vs, ctx: context.Background(), logger: newTestLogger()}
}

func TestEnsureReviewsOrg_findOrCreate(t *testing.T) {
	a := newReviewTestApp(t)
	id1, err := a.ensureReviewsOrg()
	require.NoError(t, err)
	require.NotEmpty(t, id1)
	id2, err := a.ensureReviewsOrg()
	require.NoError(t, err)
	assert.Equal(t, id1, id2, "second call reuses the same org")
}

func TestEnsurePRReviewContext_idempotent(t *testing.T) {
	a := newReviewTestApp(t)
	repo, err := a.vcsStore.Repos.EnsureRepo(a.ctx, "github", "acme", "api")
	require.NoError(t, err)

	cid1, key1, err := a.ensurePRReviewContext(repo.ID, 349, "feat/x", "My PR")
	require.NoError(t, err)
	require.NotEmpty(t, cid1)
	assert.Equal(t, "acme/api#349", key1)

	cid2, key2, err := a.ensurePRReviewContext(repo.ID, 349, "feat/x", "My PR")
	require.NoError(t, err)
	assert.Equal(t, cid1, cid2, "same PR reuses the same context")
	assert.Equal(t, key1, key2)

	// The PR branch is linked (once).
	branches, err := a.vcsStore.Branches.ListBranches(a.ctx, cid1)
	require.NoError(t, err)
	count := 0
	for _, b := range branches {
		if b.Branch == "feat/x" {
			count++
		}
	}
	assert.Equal(t, 1, count, "branch linked exactly once across re-runs")
	_ = fmt.Sprint(key2)
}

func TestEnsureReviewContexts_createsForPendingOnly(t *testing.T) {
	a := newReviewTestApp(t)
	repo, err := a.vcsStore.Repos.EnsureRepo(a.ctx, "github", "acme", "api")
	require.NoError(t, err)

	upsert := func(prNumber int, kind string) {
		t.Helper()
		_, err := a.vcsStore.ReviewRequests.UpsertReviewRequest(a.ctx, repo.ID, prNumber,
			"PR title", "author", "https://x", "", fmt.Sprintf("feat/%d", prNumber),
			"2026-10-01T00:00:00Z", "", "", kind, "", "")
		require.NoError(t, err)
	}
	upsert(1, "requested") // pending review assigned to me → context
	upsert(2, "requested") // will be dismissed → no context
	upsert(3, "authored")  // my own PR → no context

	reqs, err := a.vcsStore.ReviewRequests.ListReviewRequests(a.ctx, repo.ID)
	require.NoError(t, err)
	for _, r := range reqs {
		if r.PRNumber == 2 {
			require.NoError(t, a.vcsStore.ReviewRequests.DismissReviewRequest(a.ctx, r.ID))
		}
	}

	a.ensureReviewContexts()

	c1, err := a.db.GetContextByKey(a.ctx, "acme/api#1")
	require.NoError(t, err)
	require.NotNil(t, c1, "pending requested review gets a context")
	c2, _ := a.db.GetContextByKey(a.ctx, "acme/api#2")
	assert.Nil(t, c2, "dismissed review gets no context")
	c3, _ := a.db.GetContextByKey(a.ctx, "acme/api#3")
	assert.Nil(t, c3, "authored PR gets no context")

	// Idempotent across syncs.
	a.ensureReviewContexts()
	again, err := a.db.GetContextByKey(a.ctx, "acme/api#1")
	require.NoError(t, err)
	assert.Equal(t, c1.ID, again.ID)
}

// Review with AI on a branch someone is already working on (a worktree exists
// and a context links the branch) lands in that context: no "owner/name#N"
// review context is created, the PR branch link joins the work context.
func TestEnsurePRReviewContext_reusesWorkContextOfCheckedOutBranch(t *testing.T) {
	a := newReviewTestApp(t)
	dir := initWorktreeTestRepo(t) // main + feat/x
	repo, err := a.vcsStore.Repos.EnsureRepo(a.ctx, "github", "acme", "api")
	require.NoError(t, err)
	_, err = a.vcsStore.RepoPaths.Add(a.ctx, repo.ID, dir)
	require.NoError(t, err)
	org, err := a.db.CreateOrganization(a.ctx, "acme")
	require.NoError(t, err)
	work, err := a.db.CreateContext(a.ctx, "JH-1", org.ID)
	require.NoError(t, err)
	_, err = a.vcsStore.Branches.AddBranch(a.ctx, work.ID, repo.ID, "feat/x", "", true)
	require.NoError(t, err)

	// Nothing checked out yet → a review context is made as before.
	assert.Equal(t, "", a.workContextForBranch(repo.ID, "feat/x"))

	// Once the branch has a worktree, the review goes into the work context.
	_, err = a.ensureWorktreeForBranch(repo.ID, "feat/x")
	require.NoError(t, err)
	cid, key, err := a.ensurePRReviewContext(repo.ID, 7, "feat/x", "My PR")
	require.NoError(t, err)
	assert.Equal(t, work.ID, cid)
	assert.Equal(t, "JH-1", key)
	rc, _ := a.db.GetContextByKey(a.ctx, "acme/api#7")
	assert.Nil(t, rc, "no per-PR review context created")
}
