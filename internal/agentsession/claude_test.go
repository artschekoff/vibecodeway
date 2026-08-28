package agentsession

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectSlug(t *testing.T) {
	// Claude Code names a project directory after the cwd with every
	// non-alphanumeric character replaced by a dash.
	assert.Equal(t, "-Users-me-dev-app", ProjectSlug("/Users/me/dev/app"))
	// A worktree path has "/." in it, so the mapping yields two dashes in a row.
	assert.Equal(t, "-Users-me-dev-app--worktrees-feat-x", ProjectSlug("/Users/me/dev/app/.worktrees/feat_x"))
	assert.Equal(t, "", ProjectSlug(""))
}

// writeTranscript drops a transcript file with a fixed modification time.
func writeTranscript(t *testing.T, home, cwd, id string, mod time.Time) string {
	t.Helper()
	dir := filepath.Join(home, "projects", ProjectSlug(cwd))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	p := filepath.Join(dir, id+".jsonl")
	require.NoError(t, os.WriteFile(p, []byte("{}\n"), 0o644))
	require.NoError(t, os.Chtimes(p, mod, mod))
	return p
}

func TestLastClaudeSessionPicksTheNewestInWindow(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/me/dev/app"
	base := time.Now().Add(-time.Hour)

	writeTranscript(t, home, cwd, "old-session", base)
	writeTranscript(t, home, cwd, "new-session", base.Add(30*time.Minute))

	assert.Equal(t, "new-session", LastClaudeSession(home, cwd, base.Add(-time.Minute)))
}

func TestLastClaudeSessionIgnoresTranscriptsOlderThanSince(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/me/dev/app"
	base := time.Now().Add(-time.Hour)

	// The agent ran here, but before this terminal was ever opened — that
	// conversation belongs to some other window, not to the tab being restored.
	writeTranscript(t, home, cwd, "stale-session", base)

	assert.Equal(t, "", LastClaudeSession(home, cwd, base.Add(time.Minute)))
}

func TestLastClaudeSessionEmptyWhenAgentNeverRanThere(t *testing.T) {
	home := t.TempDir()
	assert.Equal(t, "", LastClaudeSession(home, "/Users/me/dev/untouched", time.Time{}))
	assert.Equal(t, "", LastClaudeSession("", "/Users/me/dev/app", time.Time{}))
	assert.Equal(t, "", LastClaudeSession(home, "", time.Time{}))
}

func TestLastClaudeSessionSkipsNonTranscripts(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/me/dev/app"
	base := time.Now().Add(-time.Hour)

	writeTranscript(t, home, cwd, "real-session", base)
	dir := filepath.Join(home, "projects", ProjectSlug(cwd))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir.jsonl"), 0o755))

	assert.Equal(t, "real-session", LastClaudeSession(home, cwd, time.Time{}))
}

func TestHomeHonoursClaudeConfigDir(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/custom-claude")
	assert.Equal(t, "/tmp/custom-claude", Home())
}
