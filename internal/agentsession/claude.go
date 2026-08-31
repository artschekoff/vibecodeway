// Package agentsession locates the on-disk conversations a coding agent leaves
// behind, so a terminal that is reopened can resume the one it was running.
//
// It reads the agent's own durable state rather than tracking the live process:
// Claude Code writes every session to a transcript file whose name is the
// session id, under a directory named after the working directory. That file is
// the only thing needed to reconstruct "which conversation was this terminal in".
package agentsession

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProjectSlug is how Claude Code names a project's transcript directory: the
// working directory's path with every non-alphanumeric character replaced by a
// dash. "/Users/me/dev/app" becomes "-Users-me-dev-app".
func ProjectSlug(cwd string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, cwd)
}

// Home is the directory Claude Code keeps its state in, honouring
// CLAUDE_CONFIG_DIR. Empty when the user's home directory cannot be resolved,
// which callers treat as "no agent state available".
func Home() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// LastClaudeSession returns the id of the newest Claude Code transcript written
// for cwd at or after `since`, or "" when no conversation ran there in that
// window. The id is the file name without ".jsonl" — exactly what
// `claude --resume` expects.
//
// `since` is the moment the terminal was opened, which is what keeps an old
// conversation from being attributed to a brand-new plain shell.
//
// ponytail: newest-wins, so two terminals in the same folder that both ran an
// agent resolve to the same id. Exact per-terminal attribution needs a Claude
// Code SessionStart hook keyed by a per-PTY env var (what cmux installs) — add
// that only if same-folder parallel agents turn out to be a real workflow.
func LastClaudeSession(claudeHome, cwd string, since time.Time) string {
	if claudeHome == "" || cwd == "" {
		return ""
	}
	entries, err := os.ReadDir(filepath.Join(claudeHome, "projects", ProjectSlug(cwd)))
	if err != nil {
		return "" // no directory means the agent never ran here
	}
	var best string
	var bestMod time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		if !isSafeSessionID(id) {
			// The id ends up typed verbatim into a shell as `claude --resume
			// <id>`. A real session id is always a UUID; anything else is an
			// untrusted filename (possibly planted by a prompt-injected
			// agent) and must never reach a shell.
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().Before(since) {
			continue
		}
		if info.ModTime().After(bestMod) {
			best, bestMod = id, info.ModTime()
		}
	}
	return best
}

// isSafeSessionID reports whether id is safe to interpolate into a shell
// command. Real Claude Code session ids are UUIDs, so restricting to
// alphanumerics, dashes and underscores costs nothing legitimate.
func isSafeSessionID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
