package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/artschekoff/vibecodeway/internal/agentsession"
)

func TestAgentResumeCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", home)

	cwd := "/Users/me/dev/app"
	dir := filepath.Join(home, "projects", agentsession.ProjectSlug(cwd))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sess-42.jsonl"), []byte("{}\n"), 0o644))

	a := &App{}
	since := time.Now().Add(-time.Hour).UnixMilli()

	assert.Equal(t, "claude --resume sess-42", a.AgentResumeCommand(cwd, since))
	// A folder the agent never touched yields no command, so the restored
	// terminal stays a plain shell.
	assert.Equal(t, "", a.AgentResumeCommand("/Users/me/dev/other", since))
	// A transcript older than the terminal belongs to some earlier window.
	assert.Equal(t, "", a.AgentResumeCommand(cwd, time.Now().Add(time.Hour).UnixMilli()))
}
