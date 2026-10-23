package main

import "github.com/artschekoff/vibecodeway/internal/pty"

// TerminalStart launches the OS default interactive shell in a PTY rooted at
// cwd. Output is streamed to the frontend as "term:data:<id>" events; process
// exit fires "term:exit:<id>". Returns the session id.
func (a *App) TerminalStart(cwd string) (string, error) {
	name, args := pty.DefaultShell()
	return a.ptyMgr.Start(
		cwd, name, args,
		func(id, chunk string) { a.emit("term:data:"+id, chunk) },
		func(id string, _ error) { a.emit("term:exit:" + id) },
	)
}

func (a *App) TerminalWrite(sessionID, data string) error {
	return a.ptyMgr.Write(sessionID, data)
}

func (a *App) TerminalResize(sessionID string, cols, rows int) error {
	return a.ptyMgr.Resize(sessionID, cols, rows)
}

func (a *App) TerminalClose(sessionID string) error {
	return a.ptyMgr.Close(sessionID)
}

// GetProcessStats returns Task Manager data: CPU/RSS aggregated over the app's
// own process subtree and over each live PTY session's subtree.
func (a *App) GetProcessStats() (pty.ProcessStats, error) {
	return a.ptyMgr.ProcessStats()
}
