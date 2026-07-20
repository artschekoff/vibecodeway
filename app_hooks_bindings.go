package main

import (
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

// CreateContextEventHook adds a hook for a context.
func (a *App) CreateContextEventHook(contextID, eventCategory, actionType, scriptCommand string) (*appstore.ContextEventHook, error) {
	return a.db.CreateContextEventHook(a.ctx, contextID, eventCategory, actionType, scriptCommand)
}

// ListContextEventHooks lists hooks for a context.
func (a *App) ListContextEventHooks(contextID string) ([]appstore.ContextEventHook, error) {
	return a.db.ListContextEventHooks(a.ctx, contextID)
}

// DeleteContextEventHook removes a hook by id.
func (a *App) DeleteContextEventHook(id string) error {
	return a.db.DeleteContextEventHook(a.ctx, id)
}
