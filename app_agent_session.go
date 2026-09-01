package main

import (
	"time"

	"github.com/artschekoff/vibecodeway/internal/agentsession"
)

// AgentResumeCommand returns the shell command that resumes the coding-agent
// conversation last run in cwd, or "" when none ran there since sinceUnixMs
// (the moment the calling terminal was opened).
//
// The frontend stores the result alongside the terminal's saved layout and
// types it back into the shell when the editor reopens. It is a command string
// rather than a bare session id so that supporting a second agent later is one
// map entry here instead of a branch in the UI.
func (a *App) AgentResumeCommand(cwd string, sinceUnixMs int64) string {
	id := agentsession.LastClaudeSession(agentsession.Home(), cwd, time.UnixMilli(sinceUnixMs))
	if id == "" {
		return ""
	}
	return "claude --resume " + id
}
