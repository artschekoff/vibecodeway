package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateAssignmentVersion inserts a new version. If setActive is true,
// deactivates all other versions for the same assignment first (in a transaction).
func (s *Store) CreateAssignmentVersion(ctx context.Context, personAssignmentID, body, llmPrompt string, setActive bool) (*PersonAssignmentVersion, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if setActive {
		if _, err := tx.ExecContext(ctx,
			`UPDATE person_assignment_versions SET is_active = 0 WHERE person_assignment_id = ?`,
			personAssignmentID,
		); err != nil {
			return nil, fmt.Errorf("deactivate versions: %w", err)
		}
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	active := 0
	if setActive {
		active = 1
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO person_assignment_versions (id, person_assignment_id, body, llm_prompt, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, personAssignmentID, body, llmPrompt, active, now); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &PersonAssignmentVersion{
		ID:                 id,
		PersonAssignmentID: personAssignmentID,
		Body:               body,
		LLMPrompt:          llmPrompt,
		IsActive:           setActive,
		CreatedAt:          now,
	}, nil
}

// ListAssignmentVersions returns all versions for an assignment, newest first.
func (s *Store) ListAssignmentVersions(ctx context.Context, personAssignmentID string) ([]PersonAssignmentVersion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, person_assignment_id, body, COALESCE(llm_prompt,''), is_active, created_at
		FROM person_assignment_versions
		WHERE person_assignment_id = ?
		ORDER BY created_at DESC, rowid DESC
	`, personAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var versions []PersonAssignmentVersion
	for rows.Next() {
		var v PersonAssignmentVersion
		var active int
		if err := rows.Scan(&v.ID, &v.PersonAssignmentID, &v.Body, &v.LLMPrompt, &active, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		v.IsActive = active == 1
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// SetActiveAssignmentVersion deactivates all versions for the assignment and activates the given one.
func (s *Store) SetActiveAssignmentVersion(ctx context.Context, personAssignmentID, versionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE person_assignment_versions SET is_active = 0 WHERE person_assignment_id = ?`,
		personAssignmentID,
	); err != nil {
		return fmt.Errorf("deactivate: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE person_assignment_versions SET is_active = 1 WHERE id = ? AND person_assignment_id = ?`,
		versionID, personAssignmentID,
	)
	if err != nil {
		return fmt.Errorf("activate: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("version %s: %w", versionID, ErrNotFound)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// GetActiveAssignmentVersion returns the currently active version, or (nil, nil) if none.
func (s *Store) GetActiveAssignmentVersion(ctx context.Context, personAssignmentID string) (*PersonAssignmentVersion, error) {
	var v PersonAssignmentVersion
	var active int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, person_assignment_id, body, COALESCE(llm_prompt,''), is_active, created_at
		FROM person_assignment_versions
		WHERE person_assignment_id = ? AND is_active = 1
		LIMIT 1
	`, personAssignmentID).Scan(&v.ID, &v.PersonAssignmentID, &v.Body, &v.LLMPrompt, &active, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active version: %w", err)
	}
	v.IsActive = true
	return &v, nil
}
