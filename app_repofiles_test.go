package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppGetRepoFileTree(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644))
	a := &App{}
	tree, err := a.GetRepoFileTree(root)
	require.NoError(t, err)
	assert.Contains(t, tree, "main.go")

}
