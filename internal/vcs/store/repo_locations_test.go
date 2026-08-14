package vcsstore_test

import (
	"context"
	"testing"

	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListReposWithLocations(t *testing.T) {
	_, vs := openTestDB(t)
	ctx := context.Background()

	// Repo A: two local paths + one workspace.
	repoA, err := vs.Repos.EnsureRepo(ctx, "github", "acme", "api")
	require.NoError(t, err)
	_, err = vs.RepoPaths.Add(ctx, repoA.ID, "/home/dev/src/api")
	require.NoError(t, err)
	_, err = vs.RepoPaths.Add(ctx, repoA.ID, "/home/dev/src/api-2")
	require.NoError(t, err)
	pathsA, err := vs.RepoPaths.ListForRepo(ctx, repoA.ID)
	require.NoError(t, err)
	require.NoError(t, vs.Workspaces.Create(ctx, &vcsstore.VCSWorkspace{
		RepoPathID:   pathsA[0].ID,
		RepoID:       repoA.ID,
		Branch:       "feat/x",
		WorktreePath: "/home/dev/src/api/.worktrees/feat-x",
		Status:       "active",
	}))

	// Repo B: no paths, no workspaces — must still appear with empty arrays.
	_, err = vs.Repos.EnsureRepo(ctx, "github", "acme", "web")
	require.NoError(t, err)

	got, err := vs.ListReposWithLocations(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, "api", got[0].Name)
	assert.Len(t, got[0].Paths, 2)
	require.Len(t, got[0].Workspaces, 1)
	assert.Equal(t, "feat/x", got[0].Workspaces[0].Branch)
	assert.Equal(t, "/home/dev/src/api/.worktrees/feat-x", got[0].Workspaces[0].WorktreePath)

	assert.Equal(t, "web", got[1].Name)
	assert.Empty(t, got[1].Paths)
	assert.Empty(t, got[1].Workspaces)
	assert.NotNil(t, got[1].Paths, "empty paths must be [] not nil")
	assert.NotNil(t, got[1].Workspaces, "empty workspaces must be [] not nil")
}
