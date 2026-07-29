package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initFileHistoryRepo makes a real repo with two commits to file.txt and returns
// the repo dir. Commit 1 writes "v1\n"; commit 2 writes "v2\n".
func initFileHistoryRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		require.NoError(t, cmd.Run(), "git %s", strings.Join(args, " "))
	}
	run("init")
	run("config", "user.email", "t@t")
	run("config", "user.name", "Tester")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v1\n"), 0o644))
	run("add", "file.txt")
	run("commit", "-m", "add file")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v2\n"), 0o644))
	run("commit", "-am", "change file")
	return dir
}

func TestFileHistory_returnsCommitsNewestFirst(t *testing.T) {
	a := newAutoLinkTestApp(t)
	dir := initFileHistoryRepo(t)

	commits, err := a.FileHistory(dir, "file.txt")
	require.NoError(t, err)
	require.Len(t, commits, 2)
	assert.Equal(t, "change file", commits[0].Subject) // newest first
	assert.Equal(t, "add file", commits[1].Subject)
	assert.Equal(t, "Tester", commits[0].Author)
	assert.NotEmpty(t, commits[0].Hash)
	assert.NotEmpty(t, commits[0].Date)
}

func TestFileHistory_emptyForUntracked(t *testing.T) {
	a := newAutoLinkTestApp(t)
	dir := initFileHistoryRepo(t)
	commits, err := a.FileHistory(dir, "nope.txt")
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func TestFileDiffAtCommit_beforeAndAfter(t *testing.T) {
	a := newAutoLinkTestApp(t)
	dir := initFileHistoryRepo(t)
	commits, err := a.FileHistory(dir, "file.txt")
	require.NoError(t, err)

	// Latest commit: before = v1, after = v2.
	d, err := a.FileDiffAtCommit(dir, "file.txt", commits[0].Hash)
	require.NoError(t, err)
	assert.Equal(t, "v1\n", d.Before)
	assert.Equal(t, "v2\n", d.After)

	// First commit added the file: before empty, after = v1.
	d, err = a.FileDiffAtCommit(dir, "file.txt", commits[1].Hash)
	require.NoError(t, err)
	assert.Equal(t, "", d.Before)
	assert.Equal(t, "v1\n", d.After)
}

func TestFileDiffAtCommit_rejectsBadHash(t *testing.T) {
	a := newAutoLinkTestApp(t)
	dir := initFileHistoryRepo(t)
	_, err := a.FileDiffAtCommit(dir, "file.txt", "HEAD; rm -rf /")
	require.Error(t, err)
}

func TestFileBlame_perLineAuthor(t *testing.T) {
	a := newAutoLinkTestApp(t)
	dir := initFileHistoryRepo(t)
	blame, err := a.FileBlame(dir, "file.txt")
	require.NoError(t, err)
	require.Len(t, blame, 1) // one line: "v2"
	assert.Equal(t, 1, blame[0].Line)
	assert.Equal(t, "Tester", blame[0].Author)
	assert.NotEmpty(t, blame[0].Commit)
	assert.NotEmpty(t, blame[0].Date)
}

func TestFileBlame_nilForUntracked(t *testing.T) {
	a := newAutoLinkTestApp(t)
	dir := initFileHistoryRepo(t)
	blame, err := a.FileBlame(dir, "nope.txt")
	require.NoError(t, err)
	assert.Nil(t, blame)
}
