package store

import (
	"context"
	"database/sql"
	"fmt"
)

// recordTaskCompletion upserts the completion row for one issue inside the
// caller's transaction. Callers must only invoke it on an observed open →
// terminal transition; the upsert then also handles a reopened issue being
// closed again, which keeps the newest completion timestamp.
func recordTaskCompletion(ctx context.Context, tx *sql.Tx, it BoardJiraAssignment, at string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO task_completions (jira_key, issue_type, org, status, completed_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(jira_key) DO UPDATE SET
			issue_type   = excluded.issue_type,
			org          = excluded.org,
			status       = excluded.status,
			completed_at = excluded.completed_at`,
		it.Key, it.IssueType, it.Org, it.Status, at)
	if err != nil {
		return fmt.Errorf("record task completion %s: %w", it.Key, err)
	}
	return nil
}

// ListTaskCompletionsSince returns completions with completed_at >= since
// (RFC3339 UTC string), oldest first. Bucketing into calendar days is left to
// the caller so it can happen in the user's local timezone.
func (s *Store) ListTaskCompletionsSince(ctx context.Context, since string) ([]TaskCompletion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT jira_key, issue_type, org, status, completed_at
		FROM task_completions
		WHERE completed_at >= ?
		ORDER BY completed_at ASC`, since)
	if err != nil {
		return nil, fmt.Errorf("list task completions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TaskCompletion
	for rows.Next() {
		var t TaskCompletion
		if err := rows.Scan(&t.Key, &t.IssueType, &t.Org, &t.Status, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan task completion: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
