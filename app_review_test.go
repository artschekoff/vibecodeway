package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
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
