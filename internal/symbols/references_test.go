package symbols

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real git repo in a tempdir — git grep needs one.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		require.NoError(t, exec.Command("git", append([]string{"-C", dir}, args...)...).Run())
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.ts"), []byte("const getUser = 1;\ncall(getUser);\nconst getUserById = 2;\n"), 0o644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	return dir
}

func TestReferences_wordMatchWithPreviews(t *testing.T) {
	dir := initRepo(t)
	refs, err := References(dir, "getUser", 10)
	require.NoError(t, err)
	// -w: getUserById must NOT match.
	require.Len(t, refs, 2)
	assert.Equal(t, "a.ts", refs[0].Path)
	assert.Equal(t, 1, refs[0].Line)
	assert.Equal(t, "const getUser = 1;", refs[0].Preview)
}

func TestReferences_noMatchesIsEmptyNotError(t *testing.T) {
	dir := initRepo(t)
	refs, err := References(dir, "nothingHasThisName", 10)
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestReferences_fixedStringNotRegex(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		require.NoError(t, exec.Command("git", append([]string{"-C", dir}, args...)...).Run())
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.ts"), []byte("const getXUser = 1;\nconst get.User = 2;\n"), 0o644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())

	// "." in the query must be treated literally, not as a regex wildcard:
	// it must NOT match "getXUser".
	refs, err := References(dir, "get.User", 10)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, "const get.User = 2;", refs[0].Preview)
}
