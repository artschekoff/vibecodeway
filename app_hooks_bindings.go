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

// CreateGlobalEventHook adds a global default hook.
func (a *App) CreateGlobalEventHook(eventCategory, actionType, scriptCommand string) (*appstore.GlobalEventHook, error) {
	return a.db.CreateGlobalEventHook(a.ctx, eventCategory, actionType, scriptCommand)
}

// ListGlobalEventHooks lists global default hooks.
func (a *App) ListGlobalEventHooks() ([]appstore.GlobalEventHook, error) {
	return a.db.ListGlobalEventHooks(a.ctx)
}

// DeleteGlobalEventHook removes a global hook by id.
func (a *App) DeleteGlobalEventHook(id string) error {
	return a.db.DeleteGlobalEventHook(a.ctx, id)
}
