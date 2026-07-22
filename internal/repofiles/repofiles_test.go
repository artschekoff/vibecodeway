package repofiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, root, rel, content string) {
	p := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
}

func TestFileTreeSkipsIgnoredDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main")
	writeFile(t, root, "node_modules/x/index.js", "x")
	writeFile(t, root, ".git/config", "gitdir")
	writeFile(t, root, "internal/svc.go", "package internal")

	tree, err := FileTree(root)
	require.NoError(t, err)
	assert.Contains(t, tree, "main.go")
	assert.Contains(t, tree, "internal/svc.go")
	for _, p := range tree {
		assert.NotContains(t, p, "node_modules")
		assert.NotContains(t, p, ".git")
	}
}

func TestReadFilesReturnsContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "AAA")
	writeFile(t, root, "sub/b.go", "BBB")
	got, err := ReadFiles(root, []string{"a.go", "sub/b.go"})
	require.NoError(t, err)
	assert.Equal(t, "AAA", got["a.go"])
	assert.Equal(t, "BBB", got["sub/b.go"])
}

func TestReadFilesRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.go", "AAA")
	// A secret file OUTSIDE root.
	outside := filepath.Join(filepath.Dir(root), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("SECRET"), 0o644))

	for _, bad := range []string{"../secret.txt", "/etc/passwd", "sub/../../secret.txt"} {
		_, err := ReadFiles(root, []string{bad})
		require.Error(t, err, "must reject unsafe path %q", bad)
		assert.NotContains(t, err.Error(), "SECRET")
	}
}

func TestReadFilesTruncatesLargeFile(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, maxFileBytes+500)
	for i := range big {
		big[i] = 'x'
	}
	writeFile(t, root, "big.go", string(big))
	got, err := ReadFiles(root, []string{"big.go"})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(got["big.go"]), maxFileBytes+len("…[truncated]"))
	assert.Contains(t, got["big.go"], "…[truncated]")
}
