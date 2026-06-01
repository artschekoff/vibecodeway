package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// ErrSystemStatus is returned when an operation attempts to delete a built-in system status.
var ErrSystemStatus = errors.New("cannot delete a system workflow status")

var slugRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ListWorkflowStatuses returns all statuses: system ones first (in seed order), then custom by created_at.
func (s *Store) ListWorkflowStatuses(ctx context.Context) ([]WorkflowStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, label, ai_hint, is_system, created_at
		FROM workflow_statuses
		ORDER BY is_system DESC, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list workflow statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []WorkflowStatus
	for rows.Next() {
		var st WorkflowStatus
		var isSystem int
		if err := rows.Scan(&st.ID, &st.Slug, &st.Label, &st.AIHint, &isSystem, &st.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow status: %w", err)
		}
		st.IsSystem = isSystem == 1
		result = append(result, st)
	}
	return result, rows.Err()
}

// IsValidWorkflowStatus reports whether slug matches a known status in the DB.
func (s *Store) IsValidWorkflowStatus(ctx context.Context, slug string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_statuses WHERE slug = ?`, slug).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check workflow status: %w", err)
	}
	return count > 0, nil
}

// CreateWorkflowStatus adds a new custom status. Slug must match [a-z][a-z0-9_]* and be unique.
func (s *Store) CreateWorkflowStatus(ctx context.Context, slug, label, aiHint string) (*WorkflowStatus, error) {
	if !slugRE.MatchString(slug) {
		return nil, fmt.Errorf("invalid slug %q: must match [a-z][a-z0-9_]*", slug)
	}
	st := &WorkflowStatus{
		ID:        uuid.New().String(),
		Slug:      slug,
		Label:     label,
		AIHint:    aiHint,
		IsSystem:  false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO workflow_statuses (id, slug, label, ai_hint, is_system, created_at)
		VALUES (?, ?, ?, ?, 0, ?)
	`, st.ID, st.Slug, st.Label, st.AIHint, st.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create workflow status: %w", err)
	}
	return st, nil
}

// UpdateWorkflowStatus changes the display label and ai_hint of a status (slug is immutable).
func (s *Store) UpdateWorkflowStatus(ctx context.Context, id, label, aiHint string) (*WorkflowStatus, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE workflow_statuses SET label = ?, ai_hint = ? WHERE id = ?`, label, aiHint, id)
	if err != nil {
		return nil, fmt.Errorf("update workflow status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	var st WorkflowStatus
	var isSystem int
	err = s.db.QueryRowContext(ctx, `
		SELECT id, slug, label, ai_hint, is_system, created_at FROM workflow_statuses WHERE id = ?
	`, id).Scan(&st.ID, &st.Slug, &st.Label, &st.AIHint, &isSystem, &st.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch updated workflow status: %w", err)
	}
	st.IsSystem = isSystem == 1
	return &st, nil
}

// DeleteWorkflowStatus removes a custom status and resets snapshot status to 'not_started'
// for any context snapshots that referenced it. Returns ErrSystemStatus for built-in statuses.
func (s *Store) DeleteWorkflowStatus(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete workflow status begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var slug string
	var isSystem int
	err = tx.QueryRowContext(ctx, `SELECT slug, is_system FROM workflow_statuses WHERE id = ?`, id).
		Scan(&slug, &isSystem)
	if err != nil {
		return fmt.Errorf("lookup workflow status: %w", err)
	}
	if isSystem == 1 {
		return ErrSystemStatus
	}

	// Reset snapshot status for any context that had this status set.
	if _, err := tx.ExecContext(ctx, `
		UPDATE context_snapshots SET status = 'not_started', is_stale = 1 WHERE status = ?
	`, slug); err != nil {
		return fmt.Errorf("reset snapshot status: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_statuses WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete workflow status row: %w", err)
	}

	return tx.Commit()
}
