package main

import (
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
