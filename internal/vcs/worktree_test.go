package vcs_test

import (
	"testing"

	"github.com/artschekoff/context-consolidator/internal/vcs"
	"github.com/stretchr/testify/require"
)

func TestBranchToWorktreeName(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{"main", "main"},
		{"feat/foo-bar", "feat-foo-bar"},
		{"feat/scope/detail", "feat-scope-detail"},
		{"hotfix/fix-123", "hotfix-fix-123"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, vcs.BranchToWorktreeName(tc.branch), tc.branch)
	}
}

func TestParseWorktreeList(t *testing.T) {
	input := `worktree /home/user/repo
HEAD abc123def
branch refs/heads/main

worktree /home/user/repo/.worktrees/feat-foo
HEAD 111222333
branch refs/heads/feat/foo

worktree /home/user/repo/.worktrees/detached
HEAD 999888777
detached

`
	result := vcs.ParseWorktreeListForTest([]byte(input))
	require.Len(t, result, 3)
	require.Equal(t, "/home/user/repo", result[0].Path)
	require.Equal(t, "main", result[0].Branch)
	require.Equal(t, "/home/user/repo/.worktrees/feat-foo", result[1].Path)
	require.Equal(t, "feat/foo", result[1].Branch)
	require.Equal(t, "", result[2].Branch)
}
