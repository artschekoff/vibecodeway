package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitAllowFail runs git and ignores a non-zero exit — `git merge` returns 1 when
// it stops on a conflict, which is exactly the state under test.
func gitAllowFail(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	_ = cmd.Run()
}

// conflictedRepo leaves `dir` mid-merge with c.txt conflicting: ours="mine",
// theirs="yours".
func conflictedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "--initial-branch=main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("base\n"), 0o644))
	git(t, dir, "add", "c.txt")
	git(t, dir, "commit", "-m", "base")

	git(t, dir, "checkout", "-b", "side")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("yours\n"), 0o644))
	git(t, dir, "commit", "-am", "theirs")

	git(t, dir, "checkout", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("mine\n"), 0o644))
	git(t, dir, "commit", "-am", "ours")

	gitAllowFail(t, dir, "merge", "side")
	return dir
}

// A conflicted file reported "modified" and showed up in both the staged and
// the local group, because statusWord had no case for U and splitStatus put
// "unmerged" on both sides.
func TestRepoDiff_ConflictIsReportedAsConflict(t *testing.T) {
	dir := conflictedRepo(t)

	changes, err := RepoDiff(dir)
	require.NoError(t, err)

	var got *FileChange
	for i := range changes {
		if changes[i].Path == "c.txt" {
			got = &changes[i]
			break
		}
	}
	require.NotNil(t, got, "the conflicted file must be listed")
	assert.True(t, got.Conflicted, "a file git left unmerged is a conflict")
	assert.Equal(t, "conflict", got.Status)
	// It belongs to one group, not to both staged and local.
	assert.Empty(t, got.IndexStatus)
	assert.Empty(t, got.WorkStatus)
}

func TestResolveConflict_OursAndTheirs(t *testing.T) {
	for _, tc := range []struct{ side, want string }{
		{"ours", "mine\n"},
		{"theirs", "yours\n"},
	} {
		t.Run(tc.side, func(t *testing.T) {
			dir := conflictedRepo(t)
			require.NoError(t, ResolveConflict(dir, "c.txt", tc.side))

			body, err := os.ReadFile(filepath.Join(dir, "c.txt"))
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(body))

			// Resolving also stages the file — that is what clears the conflict.
			changes, err := RepoDiff(dir)
			require.NoError(t, err)
			for _, c := range changes {
				if c.Path == "c.txt" {
					assert.False(t, c.Conflicted, "the conflict is gone once resolved")
				}
			}
		})
	}
}

func TestResolveConflict_RejectsUnknownSide(t *testing.T) {
	dir := conflictedRepo(t)
	require.Error(t, ResolveConflict(dir, "c.txt", "whatever"))
}
