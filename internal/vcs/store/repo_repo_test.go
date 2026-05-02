package vcsstore_test

import (
	"context"
	"testing"

	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRepoMeta(t *testing.T) {
	_, vcsStore := openTestDB(t) // openTestDB defined in pr_repo_test.go (same package)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "myrepo")
	require.NoError(t, err)
	assert.Equal(t, "", repo.Label)
	assert.Equal(t, "", repo.Description)

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "My Repo", "A test repository")
	require.NoError(t, err)

	updated, err := vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "My Repo", updated.Label)
	assert.Equal(t, "A test repository", updated.Description)
}

func TestUpdateRepoMetaNotFound(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	err := vcsStore.Repos.UpdateRepoMeta(ctx, "nonexistent-id", "label", "desc")
	assert.ErrorIs(t, err, vcsstore.ErrNotFound)
}
