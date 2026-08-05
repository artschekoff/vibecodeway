package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// scopeGuardScript is a Claude Code PreToolUse hook. It reads the tool call JSON
// on stdin and blocks (exit 2) any file access that resolves outside the allowed
// roots (the context's git worktrees), so the Solve-with-AI agent cannot wander
// the filesystem. Bash is best-effort: it blocks commands that reference a path
// under $HOME that isn't inside an allowed root (system paths like /usr stay
// allowed). A shell can still be tricked (var expansion, subshells) — this is a
// guardrail, not a sandbox; the prompt text carries the rest.
const scopeGuardScript = `#!/usr/bin/env python3
"""Context Consolidator scope guard (Claude Code PreToolUse hook).
Usage: cc_scope_guard.py <allowed-roots-file>
Blocks a tool call by exiting 2 (stderr is shown to the agent)."""
import json, os, re, sys

def load_roots(path):
    try:
        with open(path) as f:
            return [os.path.realpath(l.strip()) for l in f if l.strip()]
    except OSError:
        return []

def under(p, roots):
    rp = os.path.realpath(os.path.expanduser(p))
    return any(rp == r or rp.startswith(r + os.sep) for r in roots)

def deny(msg):
    sys.stderr.write("Scope guard: " + msg + "\n")
    sys.exit(2)

def main():
    roots = load_roots(sys.argv[1]) if len(sys.argv) > 1 else []
    if not roots:
        sys.exit(0)  # misconfigured -> fail open; the prompt still constrains
    try:
        data = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        sys.exit(0)
    tool = data.get("tool_name", "")
    ti = data.get("tool_input") or {}

    if tool in {"Read", "Edit", "Write", "MultiEdit", "NotebookEdit", "Glob", "Grep"}:
        for key in ("file_path", "path", "notebook_path"):
            v = ti.get(key)
            if isinstance(v, str) and v and not under(v, roots):
                deny(tool + " path is outside the allowed repositories: " + v)
        sys.exit(0)

    if tool == "Bash":
        cmd = ti.get("command", "") or ""
        home = os.path.realpath(os.path.expanduser("~"))
        for m in re.findall(r'(?:^|[\s"\x27=(:])(~/[^\s"\x27;|&)]*|/[^\s"\x27;|&)]+)', cmd):
            p = os.path.realpath(os.path.expanduser(m))
            if (p == home or p.startswith(home + os.sep)) and not under(p, roots):
                deny("Bash command references a path outside the allowed repositories: " + m)
        sys.exit(0)

    sys.exit(0)

if __name__ == "__main__":
    main()
`

// installScopeGuard installs the PreToolUse scope guard into the primary
// worktree's .claude/settings.local.json (git-ignored, so the repo stays clean),
// restricting the agent's file access to roots. The guard script and per-context
// allowed-roots file live under ~/.context-consolidator so nothing is written
// into the repo except the settings file. Best-effort: any error is returned to
// the caller, which logs and continues (the prompt still constrains the agent).
func (a *App) installScopeGuard(contextID, primaryWorktree string, roots []string) error {
	if primaryWorktree == "" || len(roots) == 0 {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	base := filepath.Join(home, ".context-consolidator")
	guardPath := filepath.Join(base, "cc_scope_guard.py")
	rootsDir := filepath.Join(base, "scope-guards")
	if err := os.MkdirAll(rootsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir scope-guards: %w", err)
	}
	if err := os.WriteFile(guardPath, []byte(scopeGuardScript), 0o755); err != nil {
		return fmt.Errorf("write guard script: %w", err)
	}
	rootsFile := filepath.Join(rootsDir, contextID+".roots")
	if err := os.WriteFile(rootsFile, []byte(strings.Join(roots, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write roots file: %w", err)
	}

	claudeDir := filepath.Join(primaryWorktree, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("mkdir .claude: %w", err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	settings := map[string]any{}
	if b, rerr := os.ReadFile(settingsPath); rerr == nil {
		_ = json.Unmarshal(b, &settings) // tolerate malformed: rebuild from scratch
	}
	cmd := fmt.Sprintf("python3 %q %q", guardPath, rootsFile)
	upsertScopeGuardHook(settings, cmd)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("write settings.local.json: %w", err)
	}
	return nil
}

// upsertScopeGuardHook merges our PreToolUse hook into a settings map in place,
// preserving any hooks the user already configured and replacing a prior copy of
// our own guard (identified by the cc_scope_guard.py marker) so re-solving is
// idempotent.
func upsertScopeGuardHook(settings map[string]any, command string) {
	const matcher = "Read|Edit|Write|MultiEdit|NotebookEdit|Glob|Grep|Bash"

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	pre, _ := hooks["PreToolUse"].([]any)

	// Drop any existing entry that references our guard script.
	kept := pre[:0:0]
	for _, e := range pre {
		if entryReferencesGuard(e) {
			continue
		}
		kept = append(kept, e)
	}
	kept = append(kept, map[string]any{
		"matcher": matcher,
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	})
	hooks["PreToolUse"] = kept
	settings["hooks"] = hooks
}

// entryReferencesGuard reports whether a PreToolUse entry invokes our guard.
func entryReferencesGuard(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	inner, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range inner {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, "cc_scope_guard.py") {
			return true
		}
	}
	return false
}
