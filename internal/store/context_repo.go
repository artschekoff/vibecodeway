package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

const (
	contextStatusActive   = "active"
	contextStatusArchived = "archived"
)

// ListContexts returns contexts ordered by updated_at desc.
// When includeArchived is false only active contexts are returned.
// LastItemAt is the newest created_at across issues, comments, and slack threads.
func (s *Store) ListContexts(ctx context.Context, includeArchived bool) ([]Context, error) {
	// Pass 1 to bypass the status filter (include all), 0 to restrict to active only.
	var includeArchivedParam int
	if includeArchived {
		includeArchivedParam = 1
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.key, c.status, c.created_at, c.updated_at,
		       COUNT(DISTINCT i.id) AS issue_count,
		       COALESCE((
		           SELECT MAX(ts) FROM (
		               SELECT MAX(i2.created_at) AS ts
		                   FROM jira_issues i2 WHERE i2.context_id = c.id
		               UNION ALL
		               SELECT MAX(jc.created_at)
		                   FROM jira_comments jc
		                   JOIN jira_issues ji ON ji.id = jc.issue_id
		                   WHERE ji.context_id = c.id
		               UNION ALL
		               SELECT MAX(st.created_at)
		                   FROM slack_threads st WHERE st.context_id = c.id
		           )
		       ), '') AS last_item_at
		FROM contexts c
		LEFT JOIN jira_issues i ON i.context_id = c.id
		WHERE (? OR c.status = 'active')
		GROUP BY c.id
		ORDER BY c.updated_at DESC
	`, includeArchivedParam)
	if err != nil {
		return nil, fmt.Errorf("listing contexts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var contexts []Context
	for rows.Next() {
		var c Context
		if err := rows.Scan(&c.ID, &c.Key, &c.Status, &c.CreatedAt, &c.UpdatedAt, &c.IssueCount, &c.LastItemAt); err != nil {
			return nil, fmt.Errorf("scanning context: %w", err)
		}
		contexts = append(contexts, c)
	}
	return contexts, rows.Err()
}

// GetContext returns a single context by ID.
func (s *Store) GetContext(ctx context.Context, id string) (*Context, error) {
	var c Context
	err := s.db.QueryRowContext(ctx,
		"SELECT id, key, status, created_at, updated_at FROM contexts WHERE id = ?",
		id,
	).Scan(&c.ID, &c.Key, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting context: %w", err)
	}
	return &c, nil
}

// GetContextByKey returns a single context by user-assigned key.
func (s *Store) GetContextByKey(ctx context.Context, key string) (*Context, error) {
	var c Context
	err := s.db.QueryRowContext(ctx,
		"SELECT id, key, status, created_at, updated_at FROM contexts WHERE key = ?",
		key,
	).Scan(&c.ID, &c.Key, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting context by key: %w", err)
	}
	return &c, nil
}

// CreateContext inserts a new context and returns it.
func (s *Store) CreateContext(ctx context.Context, key string) (*Context, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	c := &Context{
		ID:        uuid.New().String(),
		Key:       key,
		Status:    contextStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO contexts (id, key, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		c.ID, c.Key, c.Status, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating context: %w", err)
	}
	return c, nil
}

// UpdateContext updates mutable fields of an existing context.
func (s *Store) UpdateContext(ctx context.Context, id, key, status string) (*Context, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		"UPDATE contexts SET key = ?, status = ?, updated_at = ? WHERE id = ?",
		key, status, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating context: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetContext(ctx, id)
}

// DeleteContext removes a context and cascades to all child rows.
func (s *Store) DeleteContext(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM contexts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting context: %w", err)
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

// ToggleContextStatus atomically flips the status between "active" and "archived".
func (s *Store) ToggleContextStatus(ctx context.Context, id string) (*Context, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE contexts
		SET status     = CASE WHEN status = 'active' THEN 'archived' ELSE 'active' END,
		    updated_at = ?
		WHERE id = ?
	`, now, id)
	if err != nil {
		return nil, fmt.Errorf("toggling context status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetContext(ctx, id)
}

// TouchContext updates the updated_at timestamp of a context.
// Returns ErrNotFound if no row with that ID exists.
func (s *Store) TouchContext(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, "UPDATE contexts SET updated_at = ? WHERE id = ?", now, id)
	if err != nil {
		return fmt.Errorf("touching context: %w", err)
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
