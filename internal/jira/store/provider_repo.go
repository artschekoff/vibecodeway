package jirastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a requested Jira record does not exist.
var ErrNotFound = errors.New("jira record not found")

// UpsertJiraProvider creates or updates the Jira provider config for a context.
func (r *Repo) UpsertJiraProvider(ctx context.Context, contextID, searchQuery string) (*JiraProvider, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing string
	err := r.db.QueryRowContext(
		ctx,
		"SELECT id FROM jira_provider WHERE context_id = ?", contextID,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(
			ctx,
			"UPDATE jira_provider SET search_query = ?, updated_at = ? WHERE id = ?",
			searchQuery, now, existing,
		)
		if err != nil {
			return nil, fmt.Errorf("updating jira provider: %w", err)
		}
		return r.getJiraProvider(ctx, existing)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("querying jira provider: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO jira_provider (id, context_id, search_query, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, contextID, searchQuery, now, now)
	if err != nil {
		return nil, fmt.Errorf("inserting jira provider: %w", err)
	}
	return r.getJiraProvider(ctx, id)
}

// GetJiraProvider returns the Jira provider config for a context.
// Returns ErrNotFound if no config has been set.
func (r *Repo) GetJiraProvider(ctx context.Context, contextID string) (*JiraProvider, error) {
	var p JiraProvider
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, search_query, base_url, created_at, updated_at
		FROM jira_provider WHERE context_id = ?
	`, contextID).Scan(&p.ID, &p.ContextID, &p.SearchQuery, &p.BaseURL, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting jira provider: %w", err)
	}
	return &p, nil
}

// SetJiraBaseURL stores the Jira instance base URL for a context.
func (r *Repo) SetJiraBaseURL(ctx context.Context, contextID, baseURL string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx,
		"UPDATE jira_provider SET base_url = ?, updated_at = ? WHERE context_id = ?",
		baseURL, now, contextID)
	return err
}

func (r *Repo) getJiraProvider(ctx context.Context, id string) (*JiraProvider, error) {
	var p JiraProvider
	err := r.db.QueryRowContext(ctx, `
		SELECT id, context_id, search_query, base_url, created_at, updated_at
		FROM jira_provider WHERE id = ?
	`, id).Scan(&p.ID, &p.ContextID, &p.SearchQuery, &p.BaseURL, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting jira provider by id: %w", err)
	}
	return &p, nil
}
