package main

import appstore "github.com/artschekoff/vibecodeway/internal/store"

// ListTerminalCommands returns all user-configured terminal commands.
func (a *App) ListTerminalCommands() ([]appstore.TerminalCommand, error) {
	return a.db.ListTerminalCommands(a.ctx)
}

// AddTerminalCommand creates a terminal command (title + command text). When
// sendEnter is true, selecting the command also sends Enter so it runs.
func (a *App) AddTerminalCommand(title, command string, sendEnter bool) (*appstore.TerminalCommand, error) {
	return a.db.AddTerminalCommand(a.ctx, title, command, sendEnter)
}

// UpdateTerminalCommand edits a command's title, command text, and send-enter flag.
func (a *App) UpdateTerminalCommand(id, title, command string, sendEnter bool) error {
	return a.db.UpdateTerminalCommand(a.ctx, id, title, command, sendEnter)
}

// DeleteTerminalCommand removes a command by id.
func (a *App) DeleteTerminalCommand(id string) error {
	return a.db.DeleteTerminalCommand(a.ctx, id)
}
