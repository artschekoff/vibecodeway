package vcsstore_test

import (
	"context"
	"testing"

	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoPathRepo_AddAndList(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "pathrepo")
	require.NoError(t, err)

	p1, err := vcsStore.RepoPaths.Add(ctx, repo.ID, "/home/user/project")
	require.NoError(t, err)
	assert.NotEmpty(t, p1.ID)
	assert.Equal(t, repo.ID, p1.RepoID)
	assert.Equal(t, "/home/user/project", p1.Path)
	assert.NotEmpty(t, p1.CreatedAt)

	p2, err := vcsStore.RepoPaths.Add(ctx, repo.ID, "/srv/clones/project")
	require.NoError(t, err)
	assert.NotEmpty(t, p2.ID)

	paths, err := vcsStore.RepoPaths.ListForRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Len(t, paths, 2)

	// Delete one path and verify only one remains.
	err = vcsStore.RepoPaths.Delete(ctx, p1.ID)
	require.NoError(t, err)

	paths, err = vcsStore.RepoPaths.ListForRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Len(t, paths, 1)
	assert.Equal(t, p2.ID, paths[0].ID)
}

func TestRepoPathRepo_DuplicateReturnsError(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "duperepo")
	require.NoError(t, err)

	_, err = vcsStore.RepoPaths.Add(ctx, repo.ID, "/home/user/project")
	require.NoError(t, err)

	_, err = vcsStore.RepoPaths.Add(ctx, repo.ID, "/home/user/project")
	assert.Error(t, err, "adding same path twice should return an error")
}

func TestRepoPathRepo_DeleteNotFound(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	err := vcsStore.RepoPaths.Delete(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, vcsstore.ErrNotFound)
}

func TestRepoPathRepo_Get(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "getrepo")
	require.NoError(t, err)

	p, err := vcsStore.RepoPaths.Add(ctx, repo.ID, "/home/user/getrepo")
	require.NoError(t, err)

	got, err := vcsStore.RepoPaths.Get(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, repo.ID, got.RepoID)
	require.Equal(t, "/home/user/getrepo", got.Path)

	_, err = vcsStore.RepoPaths.Get(ctx, "nonexistent")
	require.ErrorIs(t, err, vcsstore.ErrNotFound)
}
