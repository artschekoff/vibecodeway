package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListContextTexts returns text entries for the given context, ordered by created_at asc.
// When activeOnly is true only rows with is_active = 1 are returned.
func (s *Store) ListContextTexts(ctx context.Context, contextID string, activeOnly bool) ([]ContextText, error) {
	q := "SELECT id, context_id, name, content, created_at, updated_at, is_active FROM context_texts WHERE context_id = ?"
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY created_at ASC"
	rows, err := s.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing context texts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var texts []ContextText
	for rows.Next() {
		var t ContextText
		if err := rows.Scan(&t.ID, &t.ContextID, &t.Name, &t.Content, &t.CreatedAt, &t.UpdatedAt, &t.IsActive); err != nil {
			return nil, fmt.Errorf("scanning context text: %w", err)
		}
		texts = append(texts, t)
	}
	return texts, rows.Err()
}

// ToggleContextTextActive flips the is_active flag for the given text entry and returns the updated record.
// Returns ErrNotFound if no entry with that ID exists.
func (s *Store) ToggleContextTextActive(ctx context.Context, id string) (*ContextText, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE context_texts SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling context text active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	var t ContextText
	err = s.db.QueryRowContext(ctx,
		"SELECT id, context_id, name, content, created_at, updated_at, is_active FROM context_texts WHERE id = ?", id,
	).Scan(&t.ID, &t.ContextID, &t.Name, &t.Content, &t.CreatedAt, &t.UpdatedAt, &t.IsActive)
	if err != nil {
		return nil, fmt.Errorf("fetching updated context text: %w", err)
	}
	return &t, nil
}

// UpsertContextText creates a new text entry when id is empty, or updates an existing one.
// Returns the resulting ContextText record.
func (s *Store) UpsertContextText(ctx context.Context, id, contextID, name, content string) (*ContextText, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	if id == "" {
		t := &ContextText{
			ID:        uuid.New().String(),
			ContextID: contextID,
			Name:      name,
			Content:   content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err := s.db.ExecContext(ctx,
			"INSERT INTO context_texts (id, context_id, name, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			t.ID, t.ContextID, t.Name, t.Content, t.CreatedAt, t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting context text: %w", err)
		}
		msgID, _ := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  t.ContextID,
			SourceID:   "text",
			ExternalID: t.ID,
			Title:      t.Name,
			Body:       t.Content,
			CreatedAt:  t.CreatedAt,
			UpdatedAt:  t.UpdatedAt,
		})
		if msgID != "" {
			_ = s.SetContextTextMessageID(ctx, t.ID, msgID)
		}
		return t, nil
	}

	res, err := s.db.ExecContext(ctx,
		"UPDATE context_texts SET name = ?, content = ?, updated_at = ? WHERE id = ?",
		name, content, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating context text: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}

	var t ContextText
	err = s.db.QueryRowContext(ctx,
		"SELECT id, context_id, name, content, created_at, updated_at, is_active FROM context_texts WHERE id = ?", id,
	).Scan(&t.ID, &t.ContextID, &t.Name, &t.Content, &t.CreatedAt, &t.UpdatedAt, &t.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching updated context text: %w", err)
	}
	msgID, _ := s.UpsertMessage(ctx, UpsertMessageArgs{
		ContextID:  t.ContextID,
		SourceID:   "text",
		ExternalID: t.ID,
		Title:      t.Name,
		Body:       t.Content,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	})
	if msgID != "" {
		_ = s.SetContextTextMessageID(ctx, t.ID, msgID)
	}
	return &t, nil
}

// SetContextTextMessageID links a context text to its messages row.
func (s *Store) SetContextTextMessageID(ctx context.Context, textID, messageID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE context_texts SET message_id = ? WHERE id = ?", messageID, textID)
	return err
}

// DeleteContextText removes a single context text by its ID.
// Returns ErrNotFound if no record with that ID exists.
func (s *Store) DeleteContextText(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM context_texts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting context text: %w", err)
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
