package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GetTaskAssignment returns the task assignment record for a context, or nil if none exists yet.
func (s *Store) GetTaskAssignment(ctx context.Context, contextID string) (*TaskAssignment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT context_id, content, generated_at, updated_at
		FROM context_task_assignments WHERE context_id = ?
	`, contextID)
	var ta TaskAssignment
	var generatedAt sql.NullString
	if err := row.Scan(&ta.ContextID, &ta.Content, &generatedAt, &ta.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task assignment: %w", err)
	}
	if generatedAt.Valid {
		ta.GeneratedAt = &generatedAt.String
	}
	return &ta, nil
}

// UpsertTaskAssignment saves or replaces a task assignment for a context.
func (s *Store) UpsertTaskAssignment(ctx context.Context, ta TaskAssignment) (*TaskAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	ta.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_task_assignments (context_id, content, generated_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(context_id) DO UPDATE SET
			content      = excluded.content,
			generated_at = excluded.generated_at,
			updated_at   = excluded.updated_at
	`, ta.ContextID, ta.Content, ta.GeneratedAt, ta.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert task assignment: %w", err)
	}
	return &ta, nil
}

// UpdateTaskAssignmentContent updates only the content field (for user manual edits).
func (s *Store) UpdateTaskAssignmentContent(ctx context.Context, contextID, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE context_task_assignments SET content = ?, updated_at = ? WHERE context_id = ?`,
		content, now, contextID,
	)
	if err != nil {
		return fmt.Errorf("update task assignment content: %w", err)
	}
	return nil
}
