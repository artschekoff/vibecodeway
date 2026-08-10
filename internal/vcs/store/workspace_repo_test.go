package vcsstore_test

import (
	"context"
	"testing"

	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"github.com/stretchr/testify/require"
)

func seedWorkspaceFixture(t *testing.T, vcsStore *vcsstore.Store) (repoID, pathID string) {
	t.Helper()
	ctx := context.Background()
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "repo")
	require.NoError(t, err)
	rp, err := vcsStore.RepoPaths.Add(ctx, repo.ID, "/tmp/repo")
	require.NoError(t, err)
	return repo.ID, rp.ID
}

func TestWorkspaceRepo_CreateAndGet(t *testing.T) {
	_, vcsStore := openTestDB(t)
	repoID, pathID := seedWorkspaceFixture(t, vcsStore)
	r := vcsStore.Workspaces
	ctx := context.Background()

	w := &vcsstore.VCSWorkspace{
		ID:           "ws-1",
		RepoPathID:   pathID,
		RepoID:       repoID,
		Branch:       "feat/foo",
		WorktreePath: "/tmp/repo/.worktrees/feat-foo",
		Status:       "active",
	}
	require.NoError(t, r.Create(ctx, w))

	got, err := r.Get(ctx, "ws-1")
	require.NoError(t, err)
	require.Equal(t, "feat/foo", got.Branch)
	require.Equal(t, "active", got.Status)
}

func TestWorkspaceRepo_FindByBranch(t *testing.T) {
	_, vcsStore := openTestDB(t)
	repoID, pathID := seedWorkspaceFixture(t, vcsStore)
	r := vcsStore.Workspaces
	ctx := context.Background()

	_ = r.Create(ctx, &vcsstore.VCSWorkspace{
		ID: "ws-1", RepoPathID: pathID, RepoID: repoID,
		Branch: "feat/foo", WorktreePath: "/tmp/repo/.worktrees/feat-foo", Status: "active",
	})

	got, err := r.FindByBranch(ctx, repoID, "feat/foo")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "ws-1", got.ID)

	missing, err := r.FindByBranch(ctx, repoID, "feat/bar")
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestWorkspaceRepo_UpdateStatus(t *testing.T) {
	_, vcsStore := openTestDB(t)
	repoID, pathID := seedWorkspaceFixture(t, vcsStore)
	r := vcsStore.Workspaces
	ctx := context.Background()

	_ = r.Create(ctx, &vcsstore.VCSWorkspace{
		ID: "ws-1", RepoPathID: pathID, RepoID: repoID,
		Branch: "feat/foo", WorktreePath: "/tmp/repo/.worktrees/feat-foo", Status: "active",
	})

	require.NoError(t, r.UpdateStatus(ctx, "ws-1", "should_delete"))
	got, _ := r.Get(ctx, "ws-1")
	require.Equal(t, "should_delete", got.Status)
}

func TestWorkspaceRepo_ListByPath(t *testing.T) {
	_, vcsStore := openTestDB(t)
	repoID, pathID := seedWorkspaceFixture(t, vcsStore)
	r := vcsStore.Workspaces
	ctx := context.Background()

	_ = r.Create(ctx, &vcsstore.VCSWorkspace{
		ID: "ws-1", RepoPathID: pathID, RepoID: repoID,
		Branch: "feat/a", WorktreePath: "/tmp/repo/.worktrees/feat-a", Status: "active",
	})
	_ = r.Create(ctx, &vcsstore.VCSWorkspace{
		ID: "ws-2", RepoPathID: pathID, RepoID: repoID,
		Branch: "feat/b", WorktreePath: "/tmp/repo/.worktrees/feat-b", Status: "should_delete",
	})

	list, err := r.ListByPath(ctx, pathID)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestWorkspaceRepo_Delete(t *testing.T) {
	_, vcsStore := openTestDB(t)
	repoID, pathID := seedWorkspaceFixture(t, vcsStore)
	r := vcsStore.Workspaces
	ctx := context.Background()

	_ = r.Create(ctx, &vcsstore.VCSWorkspace{
		ID: "ws-1", RepoPathID: pathID, RepoID: repoID,
		Branch: "feat/foo", WorktreePath: "/tmp/repo/.worktrees/feat-foo", Status: "active",
	})

	require.NoError(t, r.Delete(ctx, "ws-1"))
	_, err := r.Get(ctx, "ws-1")
	require.ErrorIs(t, err, vcsstore.ErrNotFound)
}
