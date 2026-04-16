package jirastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GetCommentsByIssue returns comments for an issue ordered by jira_created asc.
// When activeOnly is true only rows with is_active = 1 are returned.
func (r *Repo) GetCommentsByIssue(ctx context.Context, issueID string, activeOnly bool) ([]Comment, error) {
	q := `
		SELECT id, issue_id, jira_id, author, body, jira_created, created_at, is_active
		FROM jira_comments
		WHERE issue_id = ?`
	if activeOnly {
		q += " AND is_active = 1"
	}
	q += " ORDER BY jira_created ASC"
	rows, err := r.db.QueryContext(ctx, q, issueID)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.JiraID, &c.Author, &c.Body, &c.JiraCreated, &c.CreatedAt, &c.IsActive); err != nil {
			return nil, fmt.Errorf("scanning comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// ToggleCommentActive flips the is_active flag for the given comment and returns the updated record.
// Returns ErrNotFound if no comment with that ID exists.
func (r *Repo) ToggleCommentActive(ctx context.Context, id string) (*Comment, error) {
	res, err := r.db.ExecContext(ctx, "UPDATE jira_comments SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("toggling comment active: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	var c Comment
	err = r.db.QueryRowContext(ctx, `
		SELECT id, issue_id, jira_id, author, body, jira_created, created_at, is_active
		FROM jira_comments WHERE id = ?
	`, id).Scan(&c.ID, &c.IssueID, &c.JiraID, &c.Author, &c.Body, &c.JiraCreated, &c.CreatedAt, &c.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching updated comment: %w", err)
	}
	return &c, nil
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
