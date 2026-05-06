package vcsstore_test

import (
	"testing"

	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/require"
)

func TestVCSWorkspaceModel(t *testing.T) {
	w := vcsstore.VCSWorkspace{
		ID:           "id-1",
		RepoPathID:   "path-1",
		RepoID:       "repo-1",
		Branch:       "feat/foo",
		WorktreePath: "/tmp/foo/.worktrees/feat-foo",
		Status:       "active",
		CreatedAt:    "2026-01-01T00:00:00Z",
		UpdatedAt:    "2026-01-01T00:00:00Z",
	}
	require.Equal(t, "feat/foo", w.Branch)
}
