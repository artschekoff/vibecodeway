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

// GetContextRelations returns all relations originating from contextID,
// each enriched with the target context key and organization. Read-only;
// powers dependency connectors in the Activity Timeline view.
func (a *App) GetContextRelations(contextID string) ([]appstore.ContextRelation, error) {
	return a.db.ListContextRelations(a.ctx, contextID)
}
