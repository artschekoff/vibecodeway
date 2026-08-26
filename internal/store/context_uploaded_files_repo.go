package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListUploadedFiles returns uploaded files for the given context, ordered by created_at asc.
// file_content (binary) is intentionally NOT fetched here to keep the result set lean;
// use GetUploadedFileContent when the raw bytes are needed.
// When activeOnly is true, only rows with is_active = 1 are returned.
func (s *Store) ListUploadedFiles(ctx context.Context, contextID string, activeOnly bool) ([]ContextUploadedFile, error) {
	q := `SELECT id, context_id, original_name, file_type, extracted_text, file_size,
	             created_at, updated_at, is_active
	      FROM context_uploaded_files
	      WHERE context_id = ?`
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY created_at ASC"

	rows, err := s.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing uploaded files: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var files []ContextUploadedFile
	for rows.Next() {
		var f ContextUploadedFile
		if err := rows.Scan(
			&f.ID, &f.ContextID, &f.OriginalName, &f.FileType,
			&f.ExtractedText, &f.FileSize, &f.CreatedAt, &f.UpdatedAt, &f.IsActive,
		); err != nil {
			return nil, fmt.Errorf("scanning uploaded file: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// GetUploadedFileContent returns the raw binary content of a single uploaded file.
// Returns ErrNotFound if no record with that ID exists.
func (s *Store) GetUploadedFileContent(ctx context.Context, id string) ([]byte, error) {
	var content []byte
	err := s.db.QueryRowContext(
		ctx,
		"SELECT file_content FROM context_uploaded_files WHERE id = ?", id,
	).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching file content: %w", err)
	}
	return content, nil
}

// InsertUploadedFile creates a new uploaded-file record, storing both the extracted
// text and the original binary content, then returns the created record.
func (s *Store) InsertUploadedFile(ctx context.Context, contextID, originalName, fileType, extractedText string, fileContent []byte, fileSize int64) (*ContextUploadedFile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	f := &ContextUploadedFile{
		ID:            uuid.New().String(),
		ContextID:     contextID,
		OriginalName:  originalName,
		FileType:      fileType,
		ExtractedText: extractedText,
		FileContent:   fileContent,
		FileSize:      fileSize,
		CreatedAt:     now,
		UpdatedAt:     now,
		IsActive:      true,
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO context_uploaded_files
		 (id, context_id, original_name, file_type, extracted_text, file_content, file_size, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.ContextID, f.OriginalName, f.FileType, f.ExtractedText, f.FileContent, f.FileSize, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting uploaded file: %w", err)
	}
	msgID, _ := s.UpsertMessage(ctx, UpsertMessageArgs{
		ContextID:  f.ContextID,
		SourceID:   "file",
		ExternalID: f.ID,
		Title:      f.OriginalName,
		Body:       f.ExtractedText,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	})
	if msgID != "" {
		_ = s.SetUploadedFileMessageID(ctx, f.ID, msgID)
	}
	return f, nil
}

// SetUploadedFileMessageID links an uploaded file to its messages row.
func (s *Store) SetUploadedFileMessageID(ctx context.Context, fileID, messageID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE context_uploaded_files SET message_id = ? WHERE id = ?", messageID, fileID)
	return err
}

// DeleteUploadedFile removes a single uploaded file by its ID.
// Returns ErrNotFound if no record with that ID exists.
func (s *Store) DeleteUploadedFile(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM context_uploaded_files WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting uploaded file: %w", err)
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

// ToggleUploadedFileActive flips the is_active flag for the given uploaded file and returns the updated record.
// Returns ErrNotFound if no file with that ID exists.
func (s *Store) ToggleUploadedFileActive(ctx context.Context, id string) (*ContextUploadedFile, error) {
	res, err := s.db.ExecContext(ctx,
		"UPDATE context_uploaded_files SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling uploaded file active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	var f ContextUploadedFile
	err = s.db.QueryRowContext(
		ctx,
		`SELECT id, context_id, original_name, file_type, extracted_text, file_size,
		        created_at, updated_at, is_active
		 FROM context_uploaded_files WHERE id = ?`, id,
	).Scan(&f.ID, &f.ContextID, &f.OriginalName, &f.FileType,
		&f.ExtractedText, &f.FileSize, &f.CreatedAt, &f.UpdatedAt, &f.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching updated uploaded file: %w", err)
	}
	return &f, nil
}
