package main

import (
	"context"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

// ListAIPrompts returns all editable AI prompt templates with their placeholder definitions.
func (a *App) ListAIPrompts() ([]appstore.AIPrompt, error) {
	return a.db.ListAIPrompts(context.Background())
}

// UpdateAIPrompt replaces the body of a prompt template. Use {{placeholder}} tokens
// for dynamic values — see each prompt's Placeholders field for the available set.
func (a *App) UpdateAIPrompt(id, body string) error {
	return a.db.UpdateAIPrompt(context.Background(), id, body)
}

// ResetAIPrompt restores a prompt template to its factory default.
func (a *App) ResetAIPrompt(id string) error {
	return a.db.ResetAIPrompt(context.Background(), id)
}
