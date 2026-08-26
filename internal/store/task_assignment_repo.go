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
		SELECT id, context_id, name, body, generated_at, analyzed_through, is_outdated, updated_at
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
		var generatedAt, analyzedThrough sql.NullString
		var isOutdated int
		if err := rows.Scan(&pa.ID, &pa.ContextID, &pa.Name, &pa.Body,
			&generatedAt, &analyzedThrough, &isOutdated, &pa.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan person assignment: %w", err)
		}
		if generatedAt.Valid {
			pa.GeneratedAt = &generatedAt.String
		}
		if analyzedThrough.Valid {
			pa.AnalyzedThrough = &analyzedThrough.String
		}
		pa.IsOutdated = isOutdated != 0
		result = append(result, pa)
	}
	return result, rows.Err()
}

// GetPersonAssignment returns a single person assignment by ID.
func (s *Store) GetPersonAssignment(ctx context.Context, id string) (*PersonAssignment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, context_id, name, body, generated_at, analyzed_through, is_outdated, updated_at
		FROM context_person_assignments WHERE id = ?
	`, id)
	var pa PersonAssignment
	var generatedAt, analyzedThrough sql.NullString
	var isOutdated int
	if err := row.Scan(&pa.ID, &pa.ContextID, &pa.Name, &pa.Body,
		&generatedAt, &analyzedThrough, &isOutdated, &pa.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	if generatedAt.Valid {
		pa.GeneratedAt = &generatedAt.String
	}
	if analyzedThrough.Valid {
		pa.AnalyzedThrough = &analyzedThrough.String
	}
	pa.IsOutdated = isOutdated != 0
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

	if _, err := tx.ExecContext(
		ctx,
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
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE context_person_assignments SET body = ?, updated_at = ? WHERE id = ?`,
		body, now, id,
	)
	if err != nil {
		return fmt.Errorf("update person assignment body: %w", err)
	}
	return nil
}

// UpsertPersonAssignment creates or updates a person assignment identified by (contextID, name).
// If a record for this person already exists in the context it is updated in-place;
// otherwise a new row is inserted.  The saved record is returned.
func (s *Store) UpsertPersonAssignment(ctx context.Context, contextID string, pa PersonAssignment) (*PersonAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existingID string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id FROM context_person_assignments WHERE context_id = ? AND name = ?`,
		contextID, pa.Name,
	).Scan(&existingID)

	switch {
	case err == sql.ErrNoRows:
		existingID = uuid.New().String()
		if _, execErr := s.db.ExecContext(ctx, `
			INSERT INTO context_person_assignments (id, context_id, name, body, generated_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, existingID, contextID, pa.Name, pa.Body, pa.GeneratedAt, now); execErr != nil {
			return nil, fmt.Errorf("insert person assignment: %w", execErr)
		}
	case err != nil:
		return nil, fmt.Errorf("lookup person assignment: %w", err)
	default:
		if _, execErr := s.db.ExecContext(ctx, `
			UPDATE context_person_assignments SET body = ?, generated_at = ?, updated_at = ? WHERE id = ?
		`, pa.Body, pa.GeneratedAt, now, existingID); execErr != nil {
			return nil, fmt.Errorf("update person assignment: %w", execErr)
		}
	}

	return s.GetPersonAssignment(ctx, existingID)
}

// DeletePersonAssignment removes a person assignment by ID.
func (s *Store) DeletePersonAssignment(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM context_person_assignments WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("delete person assignment: %w", err)
	}
	return nil
}

// SetPersonAssignmentOutdated sets the is_outdated flag without touching body or chain steps.
func (s *Store) SetPersonAssignmentOutdated(ctx context.Context, id string, outdated bool) error {
	v := 0
	if outdated {
		v = 1
	}
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE context_person_assignments SET is_outdated = ?, updated_at = ? WHERE id = ?`,
		v, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

// SetPersonAssignmentAnalyzedThrough advances the watermark timestamp.
func (s *Store) SetPersonAssignmentAnalyzedThrough(ctx context.Context, id, ts string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE context_person_assignments SET analyzed_through = ?, updated_at = ? WHERE id = ?`,
		ts, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}
