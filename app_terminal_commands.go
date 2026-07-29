package main

import appstore "github.com/artschekoff/context-consolidator/internal/store"

// ListTerminalCommands returns all user-configured terminal commands.
func (a *App) ListTerminalCommands() ([]appstore.TerminalCommand, error) {
	return a.db.ListTerminalCommands(a.ctx)
}

// AddTerminalCommand creates a terminal command (title + command text).
func (a *App) AddTerminalCommand(title, command string) (*appstore.TerminalCommand, error) {
	return a.db.AddTerminalCommand(a.ctx, title, command)
}

// UpdateTerminalCommand edits a command's title and command text.
func (a *App) UpdateTerminalCommand(id, title, command string) error {
	return a.db.UpdateTerminalCommand(a.ctx, id, title, command)
}

// DeleteTerminalCommand removes a command by id.
func (a *App) DeleteTerminalCommand(id string) error {
	return a.db.DeleteTerminalCommand(a.ctx, id)
}
