package slackstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a requested Slack record does not exist.
var ErrNotFound = errors.New("slack record not found")

// UpsertSlackProvider creates or updates the Slack provider config for a context.
func (r *Repo) UpsertSlackProvider(ctx context.Context, contextID, workspace, searchQuery string) (*SlackProvider, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing string
	err := r.db.QueryRowContext(
		ctx,
		"SELECT id FROM slack_provider WHERE context_id = ?", contextID,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(ctx, `
			UPDATE slack_provider SET workspace = ?, search_query = ?, updated_at = ?
			WHERE id = ?
		`, workspace, searchQuery, now, existing)
		if err != nil {
			return nil, fmt.Errorf("updating slack provider: %w", err)
		}
		return r.getSlackProvider(ctx, existing)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("querying slack provider: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO slack_provider (id, context_id, workspace, search_query, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, contextID, workspace, searchQuery, now, now)
	if err != nil {
		return nil, fmt.Errorf("inserting slack provider: %w", err)
	}
	return r.getSlackProvider(ctx, id)
}

// GetSlackProvider returns the Slack provider config for a context.
// Returns ErrNotFound if no config has been set.
func (r *Repo) GetSlackProvider(ctx context.Context, contextID string) (*SlackProvider, error) {
	var p SlackProvider
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, workspace, search_query, created_at, updated_at
		FROM slack_provider WHERE context_id = ?
	`, contextID).Scan(&p.ID, &p.ContextID, &p.Workspace, &p.SearchQuery, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting slack provider: %w", err)
	}
	return &p, nil
}

func (r *Repo) getSlackProvider(ctx context.Context, id string) (*SlackProvider, error) {
	var p SlackProvider
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, workspace, search_query, created_at, updated_at
		FROM slack_provider WHERE id = ?
	`, id).Scan(&p.ID, &p.ContextID, &p.Workspace, &p.SearchQuery, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting slack provider by id: %w", err)
	}
	return &p, nil
}
