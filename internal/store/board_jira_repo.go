package store

import (
	"context"
	"database/sql"
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

// GetJiraKeyByID returns the jira_key for a jira_issues row id, or "" if the row
// is gone. Used to label timeline events with the issue key so two different issues
// that undergo the same transition don't render as identical summaries.
func (s *Store) GetJiraKeyByID(ctx context.Context, id string) (string, error) {
	var key string
	err := s.db.QueryRowContext(ctx, `SELECT jira_key FROM jira_issues WHERE id = ?`, id).Scan(&key)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get jira key by id: %w", err)
	}
	return key, nil
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

	// Snapshot the stored statuses before they are overwritten: an open →
	// terminal transition here is the only "task closed" signal the app gets,
	// since Jira's own resolution date is never fetched.
	prevStatus := make(map[string]string)
	prevRows, err := tx.QueryContext(ctx, `SELECT jira_key, status FROM board_jira_assignments`)
	if err != nil {
		return fmt.Errorf("read previous board jira statuses: %w", err)
	}
	for prevRows.Next() {
		var key, status string
		if err := prevRows.Scan(&key, &status); err != nil {
			_ = prevRows.Close()
			return fmt.Errorf("scan previous board jira status: %w", err)
		}
		prevStatus[key] = status
	}
	if err := prevRows.Err(); err != nil {
		_ = prevRows.Close()
		return fmt.Errorf("read previous board jira statuses: %w", err)
	}
	if err := prevRows.Close(); err != nil {
		return fmt.Errorf("close previous board jira statuses: %w", err)
	}

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
			INSERT INTO board_jira_assignments (jira_key, summary, status, issue_type, priority, created, org, org_checked, seen, fetched_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)
			ON CONFLICT(jira_key) DO UPDATE SET
				summary = excluded.summary,
				status = excluded.status,
				issue_type = excluded.issue_type,
				priority = excluded.priority,
				created = excluded.created,
				-- Jira search can't return the org custom field (acli allowlist), so an
				-- empty incoming org means "unknown", not "cleared": keep what we have
				-- and leave the key queued for the per-key backfill.
				org = COALESCE(NULLIF(excluded.org, ''), board_jira_assignments.org),
				org_checked = CASE WHEN excluded.org <> '' THEN 1 ELSE board_jira_assignments.org_checked END,
				fetched_at = excluded.fetched_at`,
			it.Key, it.Summary, it.Status, it.IssueType, it.Priority, it.Created, it.Org, boolToInt(it.Org != ""), now)
		if err != nil {
			return fmt.Errorf("upsert board jira %s: %w", it.Key, err)
		}

		// Only a *transition* counts. An issue that is already terminal the first
		// time we see it has no observed close date and is skipped.
		if old, seen := prevStatus[it.Key]; seen && !IsTerminalTicketStatus(old) && IsTerminalTicketStatus(it.Status) {
			if err := recordTaskCompletion(ctx, tx, it, now); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// ListBoardJiraAssignments returns the persisted assignments sorted by orderBy
// ("priority" default, or "created").
func (s *Store) ListBoardJiraAssignments(ctx context.Context, orderBy string) ([]BoardJiraAssignment, error) {
	// LEFT JOIN to surface an existing context link. A key can live in several
	// contexts (UNIQUE is per (context_id, jira_key)); pick one deterministically
	// via MIN(context_id) so each assignment yields exactly one row.
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.jira_key, b.summary, b.status, b.issue_type, b.priority, b.created, b.org, b.seen, b.fetched_at,
		       ji.context_id, c.key
		FROM board_jira_assignments b
		LEFT JOIN (SELECT jira_key, MIN(context_id) AS context_id FROM jira_issues GROUP BY jira_key) ji
		       ON ji.jira_key = b.jira_key
		LEFT JOIN contexts c ON c.id = ji.context_id
		-- Drop assignments whose (deterministically-picked) context is archived;
		-- keep unlinked assignments (no context) untouched.
		WHERE (ji.context_id IS NULL OR c.status = 'active')
		`+jiraAssignmentsOrderBy(orderBy))
	if err != nil {
		return nil, fmt.Errorf("list board jira: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BoardJiraAssignment
	for rows.Next() {
		var it BoardJiraAssignment
		var seen int
		var ctxID, ctxKey sql.NullString
		if err := rows.Scan(&it.Key, &it.Summary, &it.Status, &it.IssueType, &it.Priority, &it.Created, &it.Org, &seen, &it.FetchedAt, &ctxID, &ctxKey); err != nil {
			return nil, fmt.Errorf("scan board jira: %w", err)
		}
		it.Seen = seen != 0
		if ctxID.Valid && ctxID.String != "" {
			id, key := ctxID.String, ctxKey.String
			it.ContextID, it.ContextKey = &id, &key
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ListBoardKeysMissingOrg returns the assignment keys whose org/scope has never
// been looked up. Drives the per-key backfill: a Jira search can't return the
// custom field, so it has to be fetched one `workitem view` at a time.
func (s *Store) ListBoardKeysMissingOrg(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT jira_key FROM board_jira_assignments WHERE org_checked = 0 ORDER BY jira_key`)
	if err != nil {
		return nil, fmt.Errorf("list board keys missing org: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan board key missing org: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// SetBoardAssignmentOrg records the looked-up org for a key. An empty org is a
// valid answer ("the field isn't set on this issue") and still marks the key
// checked, so it leaves the backfill queue.
func (s *Store) SetBoardAssignmentOrg(ctx context.Context, key, org string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE board_jira_assignments SET org = ?, org_checked = 1 WHERE jira_key = ?`,
		org, key,
	); err != nil {
		return fmt.Errorf("set board org for %s: %w", key, err)
	}
	return nil
}

// MarkBoardJiraAssignmentsSeen marks every persisted assignment as seen.
func (s *Store) MarkBoardJiraAssignmentsSeen(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE board_jira_assignments SET seen = 1 WHERE seen = 0`); err != nil {
		return fmt.Errorf("mark board jira seen: %w", err)
	}
	return nil
}
