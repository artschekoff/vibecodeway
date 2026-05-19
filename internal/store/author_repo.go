package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ResolveAuthor looks up the org user ID for a (sourceID, externalName) pair.
// Returns empty string when no mapping exists — caller should leave author_id NULL.
func (s *Store) ResolveAuthor(ctx context.Context, sourceID, externalName string) string {
	if externalName == "" {
		return ""
	}
	var userID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id FROM author_mappings WHERE source_id = ? AND external_name = ?`,
		sourceID, externalName,
	).Scan(&userID)
	if err != nil {
		return ""
	}
	return userID
}

// CreateAuthorMapping records that (sourceID, externalName) belongs to userID.
// Safe to call multiple times — updates the user_id on conflict.
func (s *Store) CreateAuthorMapping(ctx context.Context, sourceID, externalName, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO author_mappings (id, user_id, source_id, external_name, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_id, external_name) DO UPDATE SET user_id = excluded.user_id
	`, id, userID, sourceID, externalName, now)
	if err != nil {
		return fmt.Errorf("create author mapping: %w", err)
	}
	return nil
}

// GetUnresolvedAuthors returns distinct (source, raw_author) pairs in contextID
// whose messages have no author_id set.
func (s *Store) GetUnresolvedAuthors(ctx context.Context, contextID string) ([]UnresolvedAuthor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.source_id, COALESCE(src.label, m.source_id), m.raw_author, COUNT(*) AS cnt
		FROM messages m
		LEFT JOIN sources src ON src.id = m.source_id
		WHERE m.context_id = ?
		  AND m.author_id IS NULL
		  AND m.raw_author != ''
		GROUP BY m.source_id, m.raw_author
		ORDER BY cnt DESC, m.raw_author ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("get unresolved authors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []UnresolvedAuthor
	for rows.Next() {
		var a UnresolvedAuthor
		if err := rows.Scan(&a.SourceID, &a.SourceLabel, &a.RawAuthor, &a.MessageCount); err != nil {
			return nil, fmt.Errorf("scan unresolved author: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AssignAuthor creates an author_mapping for (sourceID, rawAuthor) → userID
// and back-fills author_id on all matching messages.
func (s *Store) AssignAuthor(ctx context.Context, sourceID, rawAuthor, userID string) error {
	if err := s.CreateAuthorMapping(ctx, sourceID, rawAuthor, userID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE messages SET author_id = ?
		WHERE source_id = ? AND raw_author = ? AND author_id IS NULL
	`, userID, sourceID, rawAuthor)
	if err != nil {
		return fmt.Errorf("backfill author_id on messages: %w", err)
	}
	return nil
}

// AssignAuthorBatch runs AssignAuthor for each mapping in a single call.
// Used by the modal "Confirm" button to submit all assignments at once.
func (s *Store) AssignAuthorBatch(ctx context.Context, assignments []AuthorAssignment) error {
	for _, a := range assignments {
		if err := s.AssignAuthor(ctx, a.SourceID, a.RawAuthor, a.UserID); err != nil {
			return err
		}
	}
	return nil
}

// AuthorAssignment is one row of input from the resolution modal.
type AuthorAssignment struct {
	SourceID  string `json:"sourceId"`
	RawAuthor string `json:"rawAuthor"`
	UserID    string `json:"userId"`
}

// CreateUserAndAssign creates a new org user, then calls AssignAuthor.
// Returns the new User so the frontend can add it to its local list.
func (s *Store) CreateUserAndAssign(ctx context.Context, args CreateUserArgs, sourceID, rawAuthor string) (*User, error) {
	u, err := s.CreateUser(ctx, args)
	if err != nil {
		return nil, err
	}
	if err := s.AssignAuthor(ctx, sourceID, rawAuthor, u.ID); err != nil {
		return nil, err
	}
	return u, nil
}

// GetAuthorMappings returns all mappings for a user — useful for the user profile page.
func (s *Store) GetAuthorMappings(ctx context.Context, userID string) ([]AuthorMapping, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, source_id, external_name, created_at
		FROM author_mappings WHERE user_id = ? ORDER BY source_id, external_name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get author mappings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []AuthorMapping
	for rows.Next() {
		var m AuthorMapping
		if err := rows.Scan(&m.ID, &m.UserID, &m.SourceID, &m.ExternalName, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan author mapping: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
