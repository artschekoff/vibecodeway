package main

import appstore "github.com/artschekoff/context-consolidator/internal/store"

// AddContextRelation creates a typed directional link from contextID to targetID.
func (a *App) AddContextRelation(contextID, targetID, relationType string) (*appstore.ContextRelation, error) {
	return a.db.AddContextRelation(a.ctx, contextID, targetID, relationType)
}

// RemoveContextRelation deletes a context relation by ID.
func (a *App) RemoveContextRelation(id string) error {
	return a.db.RemoveContextRelation(a.ctx, id)
}
