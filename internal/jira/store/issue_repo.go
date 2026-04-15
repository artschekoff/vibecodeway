package jirastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GetIssuesByContext returns all issues for a context, ordered by created_at.
func (r *Repo) GetIssuesByContext(ctx context.Context, contextID string) ([]Issue, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, context_id, jira_key, summary, status, assignee, issue_type,
		       description, jira_updated, created_at
		FROM jira_issues
		WHERE context_id = ?
		ORDER BY created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("listing issues: %w", err)
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var i Issue
		if err := rows.Scan(
			&i.ID, &i.ContextID, &i.JiraKey, &i.Summary, &i.Status,
			&i.Assignee, &i.IssueType, &i.Description, &i.JiraUpdated, &i.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning issue: %w", err)
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

// GetIssueJiraKeys returns all jira_key values for a context (for dedup checks).
func (r *Repo) GetIssueJiraKeys(ctx context.Context, contextID string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT jira_key FROM jira_issues WHERE context_id = ?", contextID)
	if err != nil {
		return nil, fmt.Errorf("getting issue keys: %w", err)
	}
	defer rows.Close()

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
