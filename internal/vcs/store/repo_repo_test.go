package vcsstore_test

import (
	"context"
	"testing"

	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRepoMeta(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "myrepo")
	require.NoError(t, err)
	assert.Equal(t, "", repo.Label)
	assert.Equal(t, "", repo.Description)
	assert.Equal(t, "", repo.OnCheckoutCommand)

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "My Repo", "A test repository", "code .")
	require.NoError(t, err)

	updated, err := vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "My Repo", updated.Label)
	assert.Equal(t, "A test repository", updated.Description)
	assert.Equal(t, "code .", updated.OnCheckoutCommand)
}

func TestUpdateRepoMetaNotFound(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	err := vcsStore.Repos.UpdateRepoMeta(ctx, "nonexistent-id", "label", "desc", "")
	assert.ErrorIs(t, err, vcsstore.ErrNotFound)
}

func TestOnCheckoutCommandRoundTrip(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "backend")
	require.NoError(t, err)

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "", "", "claude --dangerously-skip-permissions")
	require.NoError(t, err)

	fetched, err := vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "claude --dangerously-skip-permissions", fetched.OnCheckoutCommand)

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "", "", "")
	require.NoError(t, err)

	fetched, err = vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "", fetched.OnCheckoutCommand)
}
