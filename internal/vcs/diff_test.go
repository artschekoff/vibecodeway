package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	require.NoError(t, cmd.Run(), "git %v", args)
}

func TestRepoDiff(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-m", "init")
	// modify tracked + add untracked
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new\n"), 0o644))

	changes, err := RepoDiff(dir)
	require.NoError(t, err)

	byPath := map[string]FileChange{}
	for _, c := range changes {
		byPath[c.Path] = c
	}
	require.Contains(t, byPath, "a.txt")
	assert.Equal(t, "modified", byPath["a.txt"].Status)
	assert.Contains(t, byPath["a.txt"].Diff, "+two")
	require.Contains(t, byPath, "b.txt")
	assert.Equal(t, "untracked", byPath["b.txt"].Status)
}

// A worktree/checkout of a feature branch has a clean working tree, so
// `git status` is empty — but the branch's committed changes vs the base branch
// must still surface in the panel. RepoDiff must include branch-vs-base commits.
func TestRepoDiff_BranchVsBase(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "base")

	// Feature branch with a committed change; working tree stays clean.
	git(t, dir, "checkout", "-b", "feat")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature\n"), 0o644))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "add feature")

	// Sanity: working tree is clean, so the old status-only behavior sees nothing.
	status, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(string(status)), "working tree must be clean for this test")

	changes, err := RepoDiff(dir)
	require.NoError(t, err)
	byPath := map[string]FileChange{}
	for _, c := range changes {
		byPath[c.Path] = c
	}
	require.Contains(t, byPath, "feature.txt", "branch's committed file must appear vs base")
	assert.Equal(t, "added", byPath["feature.txt"].CommitStatus)
	// base.txt is unchanged vs base — must NOT appear.
	assert.NotContains(t, byPath, "base.txt")
}

// A file that is part of the branch (committed vs base) AND has a local
// uncommitted edit must appear in BOTH views: the branch-vs-base list (the PR
// stays complete, GitHub-style) and the working-tree list (local changes). The
// two are independent — no cross-group dedup.
func TestRepoDiff_committedFileWithLocalEditAppearsInBothGroups(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("base\n"), 0o644))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "base")

	git(t, dir, "checkout", "-b", "feat")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("committed\n"), 0o644))
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-m", "feat")
	// Local uncommitted edit to a file that is already in the PR.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("locally edited\n"), 0o644))

	changes, err := RepoDiff(dir)
	require.NoError(t, err)

	var hasBranch, hasLocal bool
	for _, c := range changes {
		if c.Path != "a.txt" {
			continue
		}
		if c.CommitStatus != "" {
			hasBranch = true
		}
		if c.WorkStatus != "" || c.IndexStatus != "" {
			hasLocal = true
		}
	}
	assert.True(t, hasBranch, "a.txt must stay in the branch-vs-base (PR) list")
	assert.True(t, hasLocal, "a.txt must also appear as a local working-tree change")
}

func TestRepoDiff_IndexAndWorkStatus(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c\n"), 0o644))
	git(t, dir, "add", "a.txt", "c.txt")
	git(t, dir, "commit", "-m", "init")

	// staged-only: modify a.txt and stage it
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644))
	git(t, dir, "add", "a.txt")
	// unstaged-only: modify c.txt but do NOT stage
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c\nchanged\n"), 0o644))
	// staged + then further modified: new staged file d.txt, then edit its worktree copy
	require.NoError(t, os.WriteFile(filepath.Join(dir, "d.txt"), []byte("d\n"), 0o644))
	git(t, dir, "add", "d.txt")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "d.txt"), []byte("d\nmore\n"), 0o644))
	// untracked
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new\n"), 0o644))

	changes, err := RepoDiff(dir)
	require.NoError(t, err)
	byPath := map[string]FileChange{}
	for _, c := range changes {
		byPath[c.Path] = c
	}

	// staged only
	require.Contains(t, byPath, "a.txt")
	assert.Equal(t, "modified", byPath["a.txt"].IndexStatus)
	assert.Equal(t, "", byPath["a.txt"].WorkStatus)

	// unstaged only
	require.Contains(t, byPath, "c.txt")
	assert.Equal(t, "", byPath["c.txt"].IndexStatus)
	assert.Equal(t, "modified", byPath["c.txt"].WorkStatus)

	// staged AND unstaged
	require.Contains(t, byPath, "d.txt")
	assert.Equal(t, "added", byPath["d.txt"].IndexStatus)
	assert.Equal(t, "modified", byPath["d.txt"].WorkStatus)

	// untracked -> unstaged untracked, not staged
	require.Contains(t, byPath, "b.txt")
	assert.Equal(t, "", byPath["b.txt"].IndexStatus)
	assert.Equal(t, "untracked", byPath["b.txt"].WorkStatus)
}
