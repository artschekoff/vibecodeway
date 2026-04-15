package jirastore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GetCommentsByIssue returns all comments for an issue ordered by jira_created asc.
func (r *Repo) GetCommentsByIssue(ctx context.Context, issueID string) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, issue_id, jira_id, author, body, jira_created, created_at
		FROM jira_comments
		WHERE issue_id = ?
		ORDER BY jira_created ASC
	`, issueID)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.JiraID, &c.Author, &c.Body, &c.JiraCreated, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// DeleteComment removes a single Jira comment by its internal ID.
// Returns ErrNotFound if no row with that ID exists.
func (r *Repo) DeleteComment(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM jira_comments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting comment: %w", err)
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

// UpsertComment inserts a comment, ignoring duplicates (dedup on issue_id + jira_id).
// Returns true if a new comment was inserted.
func (r *Repo) UpsertComment(ctx context.Context, issueID, jiraID, author, body, jiraCreated string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	res, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO jira_comments
			(id, issue_id, jira_id, author, body, jira_created, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, issueID, jiraID, author, body, jiraCreated, now)
	if err != nil {
		return false, fmt.Errorf("upserting comment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("checking rows affected: %w", err)
	}
	return n > 0, nil
}
