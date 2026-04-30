package jirastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GetIssuesByContext returns issues for a context, ordered by created_at.
// When activeOnly is true only rows with is_active = 1 are returned.
func (r *Repo) GetIssuesByContext(ctx context.Context, contextID string, activeOnly bool) ([]Issue, error) {
	q := `
		SELECT id, context_id, jira_key, summary, status, assignee, issue_type,
		       description, jira_updated, created_at, is_active, viewed_at, ai_processed_at
		FROM jira_issues
		WHERE context_id = ?`
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY created_at ASC"
	rows, err := r.db.QueryContext(ctx, q, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing issues: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var issues []Issue
	for rows.Next() {
		var i Issue
		if err := rows.Scan(
			&i.ID, &i.ContextID, &i.JiraKey, &i.Summary, &i.Status,
			&i.Assignee, &i.IssueType, &i.Description, &i.JiraUpdated, &i.CreatedAt, &i.IsActive, &i.ViewedAt, &i.AiProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning issue: %w", err)
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

// ToggleIssueActive flips the is_active flag for the given issue and returns the updated record.
// Returns ErrNotFound if no issue with that ID exists.
func (r *Repo) ToggleIssueActive(ctx context.Context, id string) (*Issue, error) {
	res, err := r.db.ExecContext(ctx, "UPDATE jira_issues SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling issue active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	var i Issue
	err = r.db.QueryRowContext(ctx, `
		SELECT id, context_id, jira_key, summary, status, assignee, issue_type,
		       description, jira_updated, created_at, is_active, viewed_at, ai_processed_at
		FROM jira_issues WHERE id = ?
	`, id).Scan(
		&i.ID, &i.ContextID, &i.JiraKey, &i.Summary, &i.Status,
		&i.Assignee, &i.IssueType, &i.Description, &i.JiraUpdated, &i.CreatedAt, &i.IsActive, &i.ViewedAt, &i.AiProcessedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching updated issue: %w", err)
	}
	return &i, nil
}

// MarkIssueViewed sets viewed_at = datetime('now') for the given issue if not already viewed.
func (r *Repo) MarkIssueViewed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE jira_issues SET viewed_at = datetime('now') WHERE id = ? AND viewed_at IS NULL", id)
	return err
}

// GetIssueJiraKeys returns all jira_key values for a context (for dedup checks).
func (r *Repo) GetIssueJiraKeys(ctx context.Context, contextID string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT jira_key FROM jira_issues WHERE context_id = ?", contextID)
	if err != nil {
		return nil, fmt.Errorf("getting issue keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	keys := make(map[string]bool)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scanning issue key: %w", err)
		}
		keys[key] = true
	}
	return keys, rows.Err()
}

// DeleteIssue removes a Jira issue by its internal ID.
// All comments for this issue are cascade-deleted via the FK constraint.
// Returns ErrNotFound if no row with that ID exists.
func (r *Repo) DeleteIssue(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM jira_issues WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting issue: %w", err)
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

// UpsertIssue inserts or updates an issue (dedup on context_id + jira_key).
// Returns the internal ID of the issue (new or existing).
func (r *Repo) UpsertIssue(ctx context.Context, contextID string, jiraKey, summary, status, assignee, issueType, description, jiraUpdated string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing string
	err := r.db.QueryRowContext(ctx,
		"SELECT id FROM jira_issues WHERE context_id = ? AND jira_key = ?",
		contextID, jiraKey,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(ctx, `
			UPDATE jira_issues SET summary = ?, status = ?, assignee = ?,
			issue_type = ?, description = ?, jira_updated = ?
			WHERE id = ?
		`, summary, status, assignee, issueType, description, jiraUpdated, existing)
		if err != nil {
			return "", fmt.Errorf("updating issue: %w", err)
		}
		return existing, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("querying issue: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO jira_issues
			(id, context_id, jira_key, summary, status, assignee, issue_type, description, jira_updated, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, contextID, jiraKey, summary, status, assignee, issueType, description, jiraUpdated, now)
	if err != nil {
		return "", fmt.Errorf("inserting issue: %w", err)
	}
	return id, nil
}

// FindContextsByJiraKeys returns a map of jira_key → []context_id for the given keys
// that exist in the DB across any context. The same key may appear in multiple contexts.
// Missing keys are absent from the result. Empty input returns nil, nil.
func (r *Repo) FindContextsByJiraKeys(ctx context.Context, keys []string) (map[string][]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat(",?", len(keys))[1:]
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT jira_key, context_id FROM jira_issues WHERE jira_key IN ("+placeholders+")",
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("finding contexts by jira keys: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[string][]string)
	for rows.Next() {
		var jiraKey, contextID string
		if err := rows.Scan(&jiraKey, &contextID); err != nil {
			return nil, fmt.Errorf("scanning jira key/context: %w", err)
		}
		result[jiraKey] = append(result[jiraKey], contextID)
	}
	return result, rows.Err()
}
