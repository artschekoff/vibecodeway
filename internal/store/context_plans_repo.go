package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateContextPlan inserts a new implementation-plan document for the given
// context and person assignment, returning the resulting record.
func (s *Store) CreateContextPlan(ctx context.Context, contextID, personAssignmentID, body string) (*ContextPlan, error) {
	p := &ContextPlan{
		ID:                 uuid.New().String(),
		ContextID:          contextID,
		PersonAssignmentID: personAssignmentID,
		Body:               body,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.ExecContext(
		ctx,
		"INSERT INTO context_plans (id, context_id, person_assignment_id, body, created_at) VALUES (?, ?, ?, ?, ?)",
		p.ID, p.ContextID, p.PersonAssignmentID, p.Body, p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting context plan: %w", err)
	}
	return p, nil
}

// GetContextPlan returns a single context plan by its ID.
// Returns ErrNotFound if no record with that ID exists.
func (s *Store) GetContextPlan(ctx context.Context, id string) (*ContextPlan, error) {
	var p ContextPlan
	err := s.db.QueryRowContext(
		ctx,
		"SELECT id, context_id, person_assignment_id, body, created_at FROM context_plans WHERE id = ?", id,
	).Scan(&p.ID, &p.ContextID, &p.PersonAssignmentID, &p.Body, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching context plan: %w", err)
	}
	return &p, nil
}

// ListContextPlansByAssignment returns all plans generated for the given person
// assignment, ordered by created_at descending (most recent first).
func (s *Store) ListContextPlansByAssignment(ctx context.Context, personAssignmentID string) ([]ContextPlan, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"SELECT id, context_id, person_assignment_id, body, created_at FROM context_plans WHERE person_assignment_id = ? ORDER BY created_at DESC",
		personAssignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing context plans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var plans []ContextPlan
	for rows.Next() {
		var p ContextPlan
		if err := rows.Scan(&p.ID, &p.ContextID, &p.PersonAssignmentID, &p.Body, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning context plan: %w", err)
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
