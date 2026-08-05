package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertScopeGuardHook_mergesAndIsIdempotent(t *testing.T) {
	// Start with a user-configured hook we must preserve.
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"matcher": "Bash", "hooks": []any{
					map[string]any{"type": "command", "command": "echo user-hook"},
				}},
			},
		},
	}

	upsertScopeGuardHook(settings, `python3 "/g/cc_scope_guard.py" "/r/ctx.roots"`)
	pre := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	require.Len(t, pre, 2, "user hook preserved + guard added")

	// Re-running replaces our own entry rather than appending a duplicate.
	upsertScopeGuardHook(settings, `python3 "/g/cc_scope_guard.py" "/r/ctx.roots"`)
	pre = settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	require.Len(t, pre, 2, "idempotent: still one user + one guard")

	guards := 0
	for _, e := range pre {
		if entryReferencesGuard(e) {
			guards++
		}
	}
	assert.Equal(t, 1, guards)
}

func TestInstallScopeGuard_writesSettingsAndRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	worktree := t.TempDir()

	app := &App{}
	roots := []string{worktree, "/tmp/other-worktree"}
	require.NoError(t, app.installScopeGuard("ctx-123", worktree, roots))

	// Guard script + roots file live under ~/.context-consolidator (repo stays clean).
	guardPath := filepath.Join(home, ".context-consolidator", "cc_scope_guard.py")
	assert.FileExists(t, guardPath)
	rootsFile := filepath.Join(home, ".context-consolidator", "scope-guards", "ctx-123.roots")
	rb, err := os.ReadFile(rootsFile)
	require.NoError(t, err)
	assert.Equal(t, strings.Join(roots, "\n")+"\n", string(rb))

	// settings.local.json in the worktree wires the PreToolUse hook.
	b, err := os.ReadFile(filepath.Join(worktree, ".claude", "settings.local.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(b, &settings))
	pre := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	require.Len(t, pre, 1)
	require.True(t, entryReferencesGuard(pre[0]))
}

// TestScopeGuard_enforcesRoots runs the generated Python guard the way Claude Code
// would: feed it a tool-call JSON on stdin and assert exit 2 (blocked) for paths
// outside the roots, exit 0 (allowed) for paths inside.
func TestScopeGuard_enforcesRoots(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "worktree")
	require.NoError(t, os.MkdirAll(root, 0o755))
	guard := filepath.Join(dir, "cc_scope_guard.py")
	require.NoError(t, os.WriteFile(guard, []byte(scopeGuardScript), 0o755))
	rootsFile := filepath.Join(dir, "ctx.roots")
	require.NoError(t, os.WriteFile(rootsFile, []byte(root+"\n"), 0o644))

	run := func(input string) int {
		cmd := exec.Command(py, guard, rootsFile)
		cmd.Stdin = strings.NewReader(input)
		if err := cmd.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode()
			}
			t.Fatalf("run guard: %v", err)
		}
		return 0
	}

	inside := `{"tool_name":"Edit","tool_input":{"file_path":"` + root + `/src/a.ts"}}`
	assert.Equal(t, 0, run(inside), "edit inside a root is allowed")

	outside := `{"tool_name":"Edit","tool_input":{"file_path":"/etc/passwd"}}`
	assert.Equal(t, 2, run(outside), "edit outside all roots is blocked")

	// Bash referencing a home-dir path outside the roots is blocked.
	bashEscape := `{"tool_name":"Bash","tool_input":{"command":"cat ~/.ssh/id_rsa"}}`
	assert.Equal(t, 2, run(bashEscape), "bash reading ~/.ssh is blocked")

	// Bash touching only system paths (not under HOME) is allowed.
	bashOk := `{"tool_name":"Bash","tool_input":{"command":"ls /usr/bin"}}`
	assert.Equal(t, 0, run(bashOk), "bash on system paths is allowed")
}
