package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDedupeRepoPaths(t *testing.T) {
	in := []RepoChange{
		{RepoName: "a/b", RepoPath: "/tmp/x"},
		{RepoName: "a/b", RepoPath: "/tmp/x"}, // dup path
		{RepoName: "c/d", RepoPath: "/tmp/y"},
	}
	out := dedupeRepoPaths(in)
	require.Len(t, out, 2)
}

func TestDedupeRepoPaths_SkipsEmptyPath(t *testing.T) {
	in := []RepoChange{
		{RepoName: "a/b", RepoPath: ""},
		{RepoName: "c/d", RepoPath: "/tmp/y"},
	}
	out := dedupeRepoPaths(in)
	require.Len(t, out, 1)
	require.Equal(t, "/tmp/y", out[0].RepoPath)
}

// The real app stores repo paths in vcs_repo_paths, not the legacy
// vcs_repos.local_path column. GetContextChanges must resolve via
// vcs_repo_paths or the Changes tab is always empty.
func TestGetContextChanges_usesRepoPaths(t *testing.T) {
	app, contextID := newAppWithContext(t)

	// A real git repo on disk with an untracked file (a working-tree change).
	dir := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		require.NoError(t, cmd.Run())
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hi"), 0o644))

	repo, err := app.vcsStore.Repos.EnsureRepo(app.ctx, "github", "acme", "repoZ")
	require.NoError(t, err)
	// Path only in vcs_repo_paths; local_path stays empty (as in prod).
	_, err = app.vcsStore.RepoPaths.Add(app.ctx, repo.ID, dir)
	require.NoError(t, err)
	_, err = app.vcsStore.Branches.AddBranch(app.ctx, contextID, repo.ID, "main", "")
	require.NoError(t, err)

	changes, err := app.GetContextChanges(contextID)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, dir, changes[0].RepoPath)
	require.NotEmpty(t, changes[0].Files)
}
