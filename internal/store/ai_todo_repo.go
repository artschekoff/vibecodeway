package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListTodoItems returns the board-wide TODO NEXT list ordered by position.
func (s *Store) ListTodoItems(ctx context.Context) ([]AITodoItem, error) {
	// Resolve each jira-ref todo to its in-app context (if one exists) so the
	// board can deep-link into it. A jira_key can appear under several contexts
	// (UNIQUE is per (context_id, jira_key)); pick one deterministically via
	// MIN(context_id), mirroring GetBoardJiraAssignments. COALESCE keeps the
	// columns as "" (not NULL) for pr/review refs and unmatched keys.
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.kind, t.title, t.rationale, t.ref_type, t.ref_key, t.priority,
		       t.position, t.status, t.source, t.created_at, t.updated_at,
		       COALESCE(ji.context_id, ''), COALESCE(c.key, '')
		FROM ai_todo_items t
		LEFT JOIN (SELECT jira_key, MIN(context_id) AS context_id FROM jira_issues GROUP BY jira_key) ji
		       ON t.ref_type = 'jira' AND ji.jira_key = t.ref_key
		LEFT JOIN contexts c ON c.id = ji.context_id
		ORDER BY t.position ASC, t.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list todo items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AITodoItem
	for rows.Next() {
		var t AITodoItem
		if err := rows.Scan(&t.ID, &t.Kind, &t.Title, &t.Rationale, &t.RefType, &t.RefKey,
			&t.Priority, &t.Position, &t.Status, &t.Source, &t.CreatedAt, &t.UpdatedAt,
			&t.ContextID, &t.ContextKey); err != nil {
			return nil, fmt.Errorf("scan todo item: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddTodoItem inserts a single todo item, defaulting id/timestamps/priority/status/source.
func (s *Store) AddTodoItem(ctx context.Context, t AITodoItem) (*AITodoItem, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Priority == "" {
		t.Priority = "medium"
	}
	if t.Status == "" {
		t.Status = "pending"
	}
	if t.Source == "" {
		t.Source = "user"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_todo_items (id, kind, title, rationale, ref_type, ref_key, priority, position, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Kind, t.Title, t.Rationale, t.RefType, t.RefKey, t.Priority, t.Position, t.Status, t.Source, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("add todo item: %w", err)
	}
	return &t, nil
}

// UpdateTodoItem updates the mutable fields of a todo item.
func (s *Store) UpdateTodoItem(ctx context.Context, t AITodoItem) (*AITodoItem, error) {
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE ai_todo_items SET kind=?, title=?, rationale=?, ref_type=?, ref_key=?, priority=?, position=?, status=?, updated_at=?
		WHERE id=?
	`, t.Kind, t.Title, t.Rationale, t.RefType, t.RefKey, t.Priority, t.Position, t.Status, t.UpdatedAt, t.ID)
	if err != nil {
		return nil, fmt.Errorf("update todo item: %w", err)
	}
	return &t, nil
}

// DeleteTodoItem removes a todo item by id.
func (s *Store) DeleteTodoItem(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_todo_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete todo item: %w", err)
	}
	return nil
}

// ReorderTodoItems sets each item's position to its index in orderedIDs.
func (s *Store) ReorderTodoItems(ctx context.Context, orderedIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("reorder begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339)
	for pos, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE ai_todo_items SET position=?, updated_at=? WHERE id=?`, pos, now, id); err != nil {
			return fmt.Errorf("reorder update: %w", err)
		}
	}
	return tx.Commit()
}

// ReplaceTodoItems replaces all source='ai' items with the given set (positions
// as provided), leaving source='user' rows untouched. Run in one tx.
func (s *Store) ReplaceTodoItems(ctx context.Context, items []AITodoItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM ai_todo_items WHERE source = 'ai'`); err != nil {
		return fmt.Errorf("replace clear: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range items {
		if t.ID == "" {
			t.ID = uuid.New().String()
		}
		if t.Source == "" {
			t.Source = "ai"
		}
		if t.Priority == "" {
			t.Priority = "medium"
		}
		if t.Status == "" {
			t.Status = "pending"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ai_todo_items (id, kind, title, rationale, ref_type, ref_key, priority, position, status, source, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, t.ID, t.Kind, t.Title, t.Rationale, t.RefType, t.RefKey, t.Priority, t.Position, t.Status, t.Source, now, now); err != nil {
			return fmt.Errorf("replace insert: %w", err)
		}
	}
	return tx.Commit()
}
