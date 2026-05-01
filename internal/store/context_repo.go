package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

const (
	contextStatusActive   = "active"
	contextStatusArchived = "archived"
)

// ListContexts returns contexts ordered by updated_at desc.
// When includeArchived is false only active contexts are returned.
// LastItemAt is the newest created_at across issues, comments, and slack threads.
func (s *Store) ListContexts(ctx context.Context, includeArchived bool) ([]Context, error) {
	// Pass 1 to bypass the status filter (include all), 0 to restrict to active only.
	var includeArchivedParam int
	if includeArchived {
		includeArchivedParam = 1
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.key, c.status, c.org_id, c.created_at, c.updated_at,
		       COUNT(DISTINCT i.id) AS issue_count,
		       COALESCE((
		           SELECT MAX(ts) FROM (
		               SELECT MAX(i2.created_at) AS ts
		                   FROM jira_issues i2 WHERE i2.context_id = c.id
		               UNION ALL
		               SELECT MAX(jc.created_at)
		                   FROM jira_comments jc
		                   JOIN jira_issues ji ON ji.id = jc.issue_id
		                   WHERE ji.context_id = c.id
		               UNION ALL
		               SELECT MAX(st.created_at)
		                   FROM slack_threads st WHERE st.context_id = c.id
		           )
		       ), '') AS last_item_at,
		       (SELECT COUNT(*) FROM jira_issues WHERE context_id = c.id AND viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM jira_comments jc2
		            JOIN jira_issues ji2 ON jc2.issue_id = ji2.id
		            WHERE ji2.context_id = c.id AND jc2.viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM slack_threads WHERE context_id = c.id AND viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM slack_dm_configs WHERE context_id = c.id AND viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM vcs_pull_requests vpr
		            JOIN context_vcs_branches b ON vpr.branch_id = b.id
		            WHERE b.context_id = c.id AND vpr.viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM vcs_pr_comments vpc
		            JOIN vcs_pull_requests vpr2 ON vpc.pr_id = vpr2.id
		            JOIN context_vcs_branches b2 ON vpr2.branch_id = b2.id
		            WHERE b2.context_id = c.id AND vpc.viewed_at IS NULL)
		     + (SELECT COUNT(*) FROM notifications n
		            WHERE n.context_id = c.id AND n.read_at IS NULL)
		       AS unviewed_count,
		       EXISTS(SELECT 1 FROM jira_provider WHERE context_id = c.id) AS has_jira,
		       (EXISTS(SELECT 1 FROM slack_channels WHERE context_id = c.id)
		        OR EXISTS(SELECT 1 FROM slack_dm_configs WHERE context_id = c.id)) AS has_slack,
		       EXISTS(SELECT 1 FROM context_vcs_branches WHERE context_id = c.id) AS has_vcs,
		       (EXISTS(SELECT 1 FROM context_texts WHERE context_id = c.id)
		        OR EXISTS(SELECT 1 FROM context_uploaded_files WHERE context_id = c.id)) AS has_files,
		       c.ai_proposed_status,
		       c.ai_proposed_comment,
		       c.ai_proposed_next_step,
		       c.ai_proposed_at,
		       c.user_status_override,
		       c.user_comment_override,
		       c.user_next_step_override,
		       c.user_override_updated_at
		FROM contexts c
		LEFT JOIN jira_issues i ON i.context_id = c.id
		WHERE (? OR c.status = 'active')
		GROUP BY c.id
		ORDER BY c.sort_order ASC, c.updated_at DESC
	`, includeArchivedParam)
	if err != nil {
		return nil, fmt.Errorf("listing contexts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanContextRows(rows)
}

// GetContext returns a single context by ID.
func (s *Store) GetContext(ctx context.Context, id string) (*Context, error) {
	var c Context
	err := s.db.QueryRowContext(ctx,
		`SELECT id, key, status, created_at, updated_at,
		       ai_proposed_status, ai_proposed_comment, ai_proposed_next_step, ai_proposed_at,
		       user_status_override, user_comment_override, user_next_step_override, user_override_updated_at
		FROM contexts WHERE id = ?`,
		id,
	).Scan(&c.ID, &c.Key, &c.Status, &c.CreatedAt, &c.UpdatedAt,
		&c.AIProposedStatus, &c.AIProposedComment, &c.AIProposedNextStep, &c.AIProposedAt,
		&c.UserStatusOverride, &c.UserCommentOverride, &c.UserNextStepOverride, &c.UserOverrideUpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting context: %w", err)
	}
	return &c, nil
}

// GetContextByKey returns a single context by user-assigned key.
func (s *Store) GetContextByKey(ctx context.Context, key string) (*Context, error) {
	var c Context
	err := s.db.QueryRowContext(ctx,
		`SELECT id, key, status, created_at, updated_at,
		       ai_proposed_status, ai_proposed_comment, ai_proposed_next_step, ai_proposed_at,
		       user_status_override, user_comment_override, user_next_step_override, user_override_updated_at
		FROM contexts WHERE key = ?`,
		key,
	).Scan(&c.ID, &c.Key, &c.Status, &c.CreatedAt, &c.UpdatedAt,
		&c.AIProposedStatus, &c.AIProposedComment, &c.AIProposedNextStep, &c.AIProposedAt,
		&c.UserStatusOverride, &c.UserCommentOverride, &c.UserNextStepOverride, &c.UserOverrideUpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting context by key: %w", err)
	}
	return &c, nil
}

// CreateContext inserts a new context and returns it.
// The new context is placed at the top of the list (sort_order = -1 so it sorts before existing rows).
// A subsequent SetContextsOrder call will normalise all values.
func (s *Store) CreateContext(ctx context.Context, key string) (*Context, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	c := &Context{
		ID:               uuid.New().String(),
		Key:              key,
		Status:           contextStatusActive,
		CreatedAt:        now,
		UpdatedAt:        now,
		AIProposedStatus: "not_started",
	}
	// Place the new context first by giving it sort_order = MIN(existing) - 1.
	// Defaults to 0 if there are no existing contexts.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contexts (id, key, status, created_at, updated_at, sort_order)
		VALUES (?, ?, ?, ?, ?, COALESCE((SELECT MIN(sort_order) - 1 FROM contexts), 0))
	`, c.ID, c.Key, c.Status, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating context: %w", err)
	}
	return c, nil
}

// SetContextsOrder persists the display order of contexts.
// ids is the full ordered list of context IDs from top to bottom.
func (s *Store) SetContextsOrder(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set order tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, "UPDATE contexts SET sort_order = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("prepare set order: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for i, id := range ids {
		if _, err := stmt.ExecContext(ctx, i, id); err != nil {
			return fmt.Errorf("set sort_order for %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// UpdateContext updates mutable fields of an existing context.
func (s *Store) UpdateContext(ctx context.Context, id, key, status string) (*Context, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		"UPDATE contexts SET key = ?, status = ?, updated_at = ? WHERE id = ?",
		key, status, now, id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating context: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetContext(ctx, id)
}

// DeleteContext removes a context and cascades to all child rows.
// The FTS virtual table does not support FK cascades, so its entries are
// cleaned up explicitly before the context row is deleted.
func (s *Store) DeleteContext(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM context_search_fts WHERE context_id = ?", id); err != nil {
		return fmt.Errorf("deleting context fts entries: %w", err)
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM contexts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting context: %w", err)
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

// ToggleContextStatus atomically flips the status between "active" and "archived".
func (s *Store) ToggleContextStatus(ctx context.Context, id string) (*Context, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE contexts
		SET status     = CASE WHEN status = 'active' THEN 'archived' ELSE 'active' END,
		    updated_at = ?
		WHERE id = ?
	`, now, id)
	if err != nil {
		return nil, fmt.Errorf("toggling context status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetContext(ctx, id)
}

// GetUnviewedBreakdown returns a per-type count of unread items for a context.
func (s *Store) GetUnviewedBreakdown(ctx context.Context, contextID string) (UnviewedBreakdown, error) {
	var b UnviewedBreakdown
	err := s.db.QueryRowContext(ctx, `
		SELECT
		    (SELECT COUNT(*) FROM jira_issues WHERE context_id = ? AND viewed_at IS NULL),
		    (SELECT COUNT(*) FROM jira_comments jc
		         JOIN jira_issues ji ON jc.issue_id = ji.id
		         WHERE ji.context_id = ? AND jc.viewed_at IS NULL),
		    (SELECT COUNT(*) FROM slack_threads WHERE context_id = ? AND viewed_at IS NULL),
		    (SELECT COUNT(*) FROM slack_dm_configs WHERE context_id = ? AND viewed_at IS NULL),
		    (SELECT COUNT(*) FROM vcs_pull_requests vpr
		         JOIN context_vcs_branches b ON vpr.branch_id = b.id
		         WHERE b.context_id = ? AND vpr.viewed_at IS NULL),
		    (SELECT COUNT(*) FROM vcs_pr_comments vpc
		         JOIN vcs_pull_requests vpr2 ON vpc.pr_id = vpr2.id
		         JOIN context_vcs_branches b2 ON vpr2.branch_id = b2.id
		         WHERE b2.context_id = ? AND vpc.viewed_at IS NULL),
		    (SELECT COUNT(*) FROM notifications n WHERE n.context_id = ? AND n.read_at IS NULL)
	`, contextID, contextID, contextID, contextID, contextID, contextID, contextID).Scan(
		&b.Issues, &b.Comments, &b.Threads, &b.DMs, &b.PRs, &b.PRComments, &b.Notifications,
	)
	if err != nil {
		return b, fmt.Errorf("getting unviewed breakdown: %w", err)
	}
	b.Total = b.Issues + b.Comments + b.Threads + b.DMs + b.PRs + b.PRComments + b.Notifications
	return b, nil
}

// MarkContextAllViewed sets viewed_at = datetime('now') for every unread item in a context,
// and marks all unread notifications as read.
func (s *Store) MarkContextAllViewed(ctx context.Context, contextID string) error {
	stmts := []string{
		`UPDATE jira_issues SET viewed_at = datetime('now') WHERE context_id = ? AND viewed_at IS NULL`,
		`UPDATE jira_comments SET viewed_at = datetime('now')
		     WHERE issue_id IN (SELECT id FROM jira_issues WHERE context_id = ?) AND viewed_at IS NULL`,
		`UPDATE slack_threads SET viewed_at = datetime('now') WHERE context_id = ? AND viewed_at IS NULL`,
		`UPDATE slack_dm_configs SET viewed_at = datetime('now') WHERE context_id = ? AND viewed_at IS NULL`,
		`UPDATE vcs_pull_requests SET viewed_at = datetime('now')
		     WHERE branch_id IN (SELECT id FROM context_vcs_branches WHERE context_id = ?) AND viewed_at IS NULL`,
		`UPDATE vcs_pr_comments SET viewed_at = datetime('now')
		     WHERE pr_id IN (
		         SELECT vpr.id FROM vcs_pull_requests vpr
		         JOIN context_vcs_branches b ON vpr.branch_id = b.id
		         WHERE b.context_id = ?
		     ) AND viewed_at IS NULL`,
		`UPDATE notifications SET read_at = datetime('now') WHERE context_id = ? AND read_at IS NULL`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q, contextID); err != nil {
			return fmt.Errorf("marking context all viewed: %w", err)
		}
	}
	return nil
}

// RebuildSearchIndex rebuilds the FTS5 full-text search index from all searchable sources.
func (s *Store) RebuildSearchIndex(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM context_search_fts`); err != nil {
		return fmt.Errorf("clear search index: %w", err)
	}
	stmts := []string{
		`INSERT INTO context_search_fts (context_id, source, body) SELECT id, 'key', key FROM contexts`,
		`INSERT INTO context_search_fts (context_id, source, body) SELECT context_id, 'jira', summary FROM jira_issues WHERE summary != ''`,
		`INSERT INTO context_search_fts (context_id, source, body) SELECT context_id, 'slack', channel_name FROM slack_channels WHERE channel_name != ''`,
		`INSERT INTO context_search_fts (context_id, source, body) SELECT context_id, 'branch', branch FROM context_vcs_branches WHERE branch != ''`,
		`INSERT INTO context_search_fts (context_id, source, body) SELECT context_id, 'text', name FROM context_texts WHERE name != ''`,
		`INSERT INTO context_search_fts (context_id, source, body) SELECT context_id, 'file', original_name FROM context_uploaded_files WHERE original_name != ''`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("populate search index: %w", err)
		}
	}
	return nil
}

// SearchContextIDs returns IDs of contexts matching the query using FTS5 with BM25 ranking.
// Searches across context keys, Jira issue summaries, Slack channel names,
// VCS branch names, context text names, and uploaded file names.
// Returns all context IDs when query is empty.
func (s *Store) SearchContextIDs(ctx context.Context, query string) ([]string, error) {
	if query == "" {
		rows, err := s.db.QueryContext(ctx, `SELECT id FROM contexts ORDER BY updated_at DESC`)
		if err != nil {
			return nil, fmt.Errorf("search contexts: %w", err)
		}
		defer func() { _ = rows.Close() }()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}

	if err := s.RebuildSearchIndex(ctx); err != nil {
		return nil, err
	}

	// FTS5 query: quote each token and append * for prefix matching.
	ftsQuery := fts5PrefixQuery(query)

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT context_id
		FROM context_search_fts
		WHERE context_search_fts MATCH ?
		ORDER BY bm25(context_search_fts)
	`, ftsQuery)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// scanContextRows scans a *sql.Rows result set into []Context.
// Column order must match the SELECT in ListContexts and ListContextsByOrg.
func scanContextRows(rows *sql.Rows) ([]Context, error) {
	var contexts []Context
	for rows.Next() {
		var c Context
		if err := rows.Scan(
			&c.ID, &c.Key, &c.Status, &c.OrgID, &c.CreatedAt, &c.UpdatedAt,
			&c.IssueCount, &c.LastItemAt, &c.UnviewedCount,
			&c.HasJira, &c.HasSlack, &c.HasVCS, &c.HasFiles,
			&c.AIProposedStatus, &c.AIProposedComment, &c.AIProposedNextStep, &c.AIProposedAt,
			&c.UserStatusOverride, &c.UserCommentOverride, &c.UserNextStepOverride, &c.UserOverrideUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning context: %w", err)
		}
		contexts = append(contexts, c)
	}
	return contexts, rows.Err()
}

// fts5PrefixQuery converts a user query like "batch met" into FTS5 syntax "batch" * AND "met" *.
// Each whitespace-separated token becomes a prefix term.
func fts5PrefixQuery(input string) string {
	words := strings.Fields(input)
	parts := make([]string, 0, len(words))
	for _, w := range words {
		escaped := strings.ReplaceAll(w, `"`, `""`)
		parts = append(parts, `"`+escaped+`"*`)
	}
	return strings.Join(parts, " AND ")
}

// TouchContext updates the updated_at timestamp of a context.
// Returns ErrNotFound if no row with that ID exists.
func (s *Store) TouchContext(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, "UPDATE contexts SET updated_at = ? WHERE id = ?", now, id)
	if err != nil {
		return fmt.Errorf("touching context: %w", err)
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
