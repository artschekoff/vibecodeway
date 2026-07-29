package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// jiraAssignmentsOrderBy maps a whitelisted sort key to a constant ORDER BY
// clause. Never interpolate caller input into SQL.
func jiraAssignmentsOrderBy(orderBy string) string {
	switch orderBy {
	case "created":
		return "ORDER BY created DESC"
	default: // "priority"
		return `ORDER BY CASE priority
			WHEN 'Highest' THEN 5 WHEN 'High' THEN 4 WHEN 'Medium' THEN 3
			WHEN 'Low' THEN 2 WHEN 'Lowest' THEN 1 ELSE 0 END DESC, created DESC`
	}
}

// UpsertBoardJiraAssignments replaces the persisted assignment set with issues,
// preserving the seen flag for keys that already existed. Keys absent from
// issues are deleted; brand-new keys default to seen = 0.
func (s *Store) UpsertBoardJiraAssignments(ctx context.Context, issues []BoardJiraAssignment) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert board jira: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete keys no longer assigned.
	if len(issues) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM board_jira_assignments`); err != nil {
			return fmt.Errorf("clear board jira: %w", err)
		}
	} else {
		placeholders := make([]string, len(issues))
		args := make([]any, len(issues))
		for i, it := range issues {
			placeholders[i] = "?"
			args[i] = it.Key
		}
		q := `DELETE FROM board_jira_assignments WHERE jira_key NOT IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("prune board jira: %w", err)
		}
	}

	for _, it := range issues {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO board_jira_assignments (jira_key, summary, status, issue_type, priority, created, seen, fetched_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, ?)
			ON CONFLICT(jira_key) DO UPDATE SET
				summary = excluded.summary,
				status = excluded.status,
				issue_type = excluded.issue_type,
				priority = excluded.priority,
				created = excluded.created,
				fetched_at = excluded.fetched_at`,
			it.Key, it.Summary, it.Status, it.IssueType, it.Priority, it.Created, now)
		if err != nil {
			return fmt.Errorf("upsert board jira %s: %w", it.Key, err)
		}
	}
	return tx.Commit()
}

// ListBoardJiraAssignments returns the persisted assignments sorted by orderBy
// ("priority" default, or "created").
func (s *Store) ListBoardJiraAssignments(ctx context.Context, orderBy string) ([]BoardJiraAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT jira_key, summary, status, issue_type, priority, created, seen, fetched_at
		FROM board_jira_assignments `+jiraAssignmentsOrderBy(orderBy))
	if err != nil {
		return nil, fmt.Errorf("list board jira: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BoardJiraAssignment
	for rows.Next() {
		var it BoardJiraAssignment
		var seen int
		if err := rows.Scan(&it.Key, &it.Summary, &it.Status, &it.IssueType, &it.Priority, &it.Created, &seen, &it.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan board jira: %w", err)
		}
		it.Seen = seen != 0
		out = append(out, it)
	}
	return out, rows.Err()
}

// MarkBoardJiraAssignmentsSeen marks every persisted assignment as seen.
func (s *Store) MarkBoardJiraAssignmentsSeen(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE board_jira_assignments SET seen = 1 WHERE seen = 0`); err != nil {
		return fmt.Errorf("mark board jira seen: %w", err)
	}
	return nil
}
