package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListContextRelations returns all relations originating from contextID, enriched with the target context key.
func (s *Store) ListContextRelations(ctx context.Context, contextID string) ([]ContextRelation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cr.id, cr.context_id, cr.target_id, c.key, c.org_id, COALESCE(o.name, ''), cr.relation_type, cr.created_at
		FROM context_relations cr
		JOIN contexts c ON c.id = cr.target_id
		LEFT JOIN organizations o ON o.id = c.org_id
		WHERE cr.context_id = ?
		ORDER BY cr.created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing context relations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var relations []ContextRelation
	for rows.Next() {
		var r ContextRelation
		if err := rows.Scan(&r.ID, &r.ContextID, &r.TargetID, &r.TargetKey, &r.TargetOrgID, &r.TargetOrgName, &r.RelationType, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning context relation: %w", err)
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// AddContextRelation inserts a relation from contextID to targetID with the given type.
// Returns the created relation with TargetKey populated.
func (s *Store) AddContextRelation(ctx context.Context, contextID, targetID, relationType string) (*ContextRelation, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	r := &ContextRelation{
		ID:           uuid.New().String(),
		ContextID:    contextID,
		TargetID:     targetID,
		RelationType: relationType,
		CreatedAt:    now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_relations (id, context_id, target_id, relation_type, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, r.ID, r.ContextID, r.TargetID, r.RelationType, r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("adding context relation: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT c.key, c.org_id, COALESCE(o.name, '')
		FROM contexts c
		LEFT JOIN organizations o ON o.id = c.org_id
		WHERE c.id = ?
	`, targetID).Scan(&r.TargetKey, &r.TargetOrgID, &r.TargetOrgName); err != nil {
		return nil, fmt.Errorf("fetching target key: %w", err)
	}
	return r, nil
}

// RemoveContextRelation deletes a relation by ID. Returns ErrNotFound if no row matches.
func (s *Store) RemoveContextRelation(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM context_relations WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("removing context relation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
