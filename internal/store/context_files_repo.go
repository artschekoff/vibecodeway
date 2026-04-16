package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListContextFiles returns text files for the given context, ordered by created_at asc.
// When activeOnly is true only rows with is_active = 1 are returned.
func (s *Store) ListContextFiles(ctx context.Context, contextID string, activeOnly bool) ([]ContextFile, error) {
	q := "SELECT id, context_id, name, content, created_at, updated_at, is_active FROM context_files WHERE context_id = ?"
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY created_at ASC"
	rows, err := s.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing context files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var files []ContextFile
	for rows.Next() {
		var f ContextFile
		if err := rows.Scan(&f.ID, &f.ContextID, &f.Name, &f.Content, &f.CreatedAt, &f.UpdatedAt, &f.IsActive); err != nil {
			return nil, fmt.Errorf("scanning context file: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// ToggleContextFileActive flips the is_active flag for the given file and returns the updated record.
// Returns ErrNotFound if no file with that ID exists.
func (s *Store) ToggleContextFileActive(ctx context.Context, id string) (*ContextFile, error) {
	res, err := s.db.ExecContext(ctx, "UPDATE context_files SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling context file active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	var f ContextFile
	err = s.db.QueryRowContext(ctx,
		"SELECT id, context_id, name, content, created_at, updated_at, is_active FROM context_files WHERE id = ?", id,
	).Scan(&f.ID, &f.ContextID, &f.Name, &f.Content, &f.CreatedAt, &f.UpdatedAt, &f.IsActive)
	if err != nil {
		return nil, fmt.Errorf("fetching updated context file: %w", err)
	}
	return &f, nil
}

// UpsertContextFile creates a new text file when id is empty, or updates an existing one.
// Returns the resulting ContextFile record.
func (s *Store) UpsertContextFile(ctx context.Context, id, contextID, name, content string) (*ContextFile, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	if id == "" {
		f := &ContextFile{
			ID:        uuid.New().String(),
			ContextID: contextID,
			Name:      name,
			Content:   content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err := s.db.ExecContext(ctx,
			"INSERT INTO context_files (id, context_id, name, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			f.ID, f.ContextID, f.Name, f.Content, f.CreatedAt, f.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("inserting context file: %w", err)
		}
		return f, nil
	}

	res, err := s.db.ExecContext(ctx,
		"UPDATE context_files SET name = ?, content = ?, updated_at = ? WHERE id = ?",
		name, content, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating context file: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}

	var f ContextFile
	err = s.db.QueryRowContext(ctx,
		"SELECT id, context_id, name, content, created_at, updated_at, is_active FROM context_files WHERE id = ?", id,
	).Scan(&f.ID, &f.ContextID, &f.Name, &f.Content, &f.CreatedAt, &f.UpdatedAt, &f.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching updated context file: %w", err)
	}
	return &f, nil
}

// DeleteContextFile removes a single context file by its ID.
// Returns ErrNotFound if no record with that ID exists.
func (s *Store) DeleteContextFile(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM context_files WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting context file: %w", err)
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
