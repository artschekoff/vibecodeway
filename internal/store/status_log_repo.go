package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AppendStatusLog appends a new status log entry for a context.
// prevID should be the ID of the current latest entry, nil for the first entry.
func (s *Store) AppendStatusLog(ctx context.Context, contextID, statusSlug, comment, source string, prevID *string) (*StatusLogEntry, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_status_log (id, context_id, status_slug, comment, source, prev_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, contextID, statusSlug, comment, source, prevID, now)
	if err != nil {
		return nil, fmt.Errorf("append status log: %w", err)
	}
	return &StatusLogEntry{
		ID:         id,
		ContextID:  contextID,
		StatusSlug: statusSlug,
		Comment:    comment,
		Source:     source,
		PrevID:     prevID,
		CreatedAt:  now,
	}, nil
}

// GetLatestStatusLogEntry returns the most recent log entry for a context, or nil if none.
func (s *Store) GetLatestStatusLogEntry(ctx context.Context, contextID string) (*StatusLogEntry, error) {
	var e StatusLogEntry
	var prevID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, context_id, status_slug, comment, source, prev_id, created_at
		FROM context_status_log
		WHERE context_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, contextID).Scan(&e.ID, &e.ContextID, &e.StatusSlug, &e.Comment, &e.Source, &prevID, &e.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest status log: %w", err)
	}
	if prevID.Valid {
		e.PrevID = &prevID.String
	}
	return &e, nil
}

// ListStatusLog returns all log entries for a context in chronological order.
func (s *Store) ListStatusLog(ctx context.Context, contextID string) ([]StatusLogEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, status_slug, comment, source, prev_id, created_at
		FROM context_status_log
		WHERE context_id = ?
		ORDER BY created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list status log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []StatusLogEntry
	for rows.Next() {
		var e StatusLogEntry
		var prevID sql.NullString
		if err := rows.Scan(&e.ID, &e.ContextID, &e.StatusSlug, &e.Comment, &e.Source, &prevID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan status log: %w", err)
		}
		if prevID.Valid {
			e.PrevID = &prevID.String
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
