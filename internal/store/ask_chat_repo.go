package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AskChat is one saved Ask AI conversation. LogJSON is the frontend's chat log
// (user / assistant+steps / summary entries) stored verbatim; the store never
// interprets it. List results leave LogJSON empty.
type AskChat struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	LogJSON   string `json:"logJson,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// UpsertAskChat creates or overwrites a chat's log and bumps updated_at.
func (s *Store) UpsertAskChat(ctx context.Context, id, title, logJSON string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ask_chats (id, title, log_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title      = excluded.title,
			log_json   = excluded.log_json,
			updated_at = excluded.updated_at`,
		id, title, logJSON, now, now)
	if err != nil {
		return fmt.Errorf("upsert ask chat: %w", err)
	}
	return nil
}

// ListAskChats returns every saved chat, newest first, without log bodies.
func (s *Store) ListAskChats(ctx context.Context) ([]AskChat, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, created_at, updated_at FROM ask_chats ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list ask chats: %w", err)
	}
	defer rows.Close()
	var out []AskChat
	for rows.Next() {
		var c AskChat
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ask chat: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetAskChat returns one chat including its log. ErrNotFound if missing.
func (s *Store) GetAskChat(ctx context.Context, id string) (*AskChat, error) {
	var c AskChat
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, log_json, created_at, updated_at FROM ask_chats WHERE id = ?`, id).
		Scan(&c.ID, &c.Title, &c.LogJSON, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ask chat: %w", err)
	}
	return &c, nil
}

// DeleteAskChat removes one chat. No error if it does not exist.
func (s *Store) DeleteAskChat(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ask_chats WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete ask chat: %w", err)
	}
	return nil
}

// PruneAskChats deletes chats not updated within retentionDays. Returns the
// number removed. retentionDays <= 0 keeps everything.
func (s *Store) PruneAskChats(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `DELETE FROM ask_chats WHERE updated_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune ask chats: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
