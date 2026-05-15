package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListPersonAssignments returns all person assignments for a context ordered by name.
func (s *Store) ListPersonAssignments(ctx context.Context, contextID string) ([]PersonAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, name, body, generated_at, updated_at
		FROM context_person_assignments
		WHERE context_id = ?
		ORDER BY name ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list person assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []PersonAssignment
	for rows.Next() {
		var pa PersonAssignment
		var generatedAt sql.NullString
		if err := rows.Scan(&pa.ID, &pa.ContextID, &pa.Name, &pa.Body, &generatedAt, &pa.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan person assignment: %w", err)
		}
		if generatedAt.Valid {
			pa.GeneratedAt = &generatedAt.String
		}
		result = append(result, pa)
	}
	return result, rows.Err()
}

// GetPersonAssignment returns a single person assignment by ID.
func (s *Store) GetPersonAssignment(ctx context.Context, id string) (*PersonAssignment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, context_id, name, body, generated_at, updated_at
		FROM context_person_assignments WHERE id = ?
	`, id)
	var pa PersonAssignment
	var generatedAt sql.NullString
	if err := row.Scan(&pa.ID, &pa.ContextID, &pa.Name, &pa.Body, &generatedAt, &pa.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	if generatedAt.Valid {
		pa.GeneratedAt = &generatedAt.String
	}
	return &pa, nil
}

// ReplacePersonAssignments deletes all existing assignments for a context and inserts the new set.
// Runs in a single transaction. Returns the inserted rows with their generated IDs.
func (s *Store) ReplacePersonAssignments(ctx context.Context, contextID string, assignments []PersonAssignment) ([]PersonAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replace person assignments: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM context_person_assignments WHERE context_id = ?`, contextID,
	); err != nil {
		return nil, fmt.Errorf("delete old person assignments: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO context_person_assignments (id, context_id, name, body, generated_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert person assignment: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	result := make([]PersonAssignment, 0, len(assignments))
	for _, a := range assignments {
		a.ID = uuid.New().String()
		a.ContextID = contextID
		a.UpdatedAt = now
		if _, err := stmt.ExecContext(ctx, a.ID, a.ContextID, a.Name, a.Body, a.GeneratedAt, a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("insert person assignment %q: %w", a.Name, err)
		}
		result = append(result, a)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace person assignments: %w", err)
	}
	return result, nil
}

// UpdatePersonAssignmentBody updates the body of a single person assignment (user manual edit).
func (s *Store) UpdatePersonAssignmentBody(ctx context.Context, id, body string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE context_person_assignments SET body = ?, updated_at = ? WHERE id = ?`,
		body, now, id,
	)
	if err != nil {
		return fmt.Errorf("update person assignment body: %w", err)
	}
	return nil
}

// DeletePersonAssignment removes a person assignment by ID.
func (s *Store) DeletePersonAssignment(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM context_person_assignments WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("delete person assignment: %w", err)
	}
	return nil
}
