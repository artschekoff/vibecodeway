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

// ArchiveContexts sets status='archived' for each of the given context ids in a
// single transaction. Already-archived contexts are skipped. Returns the number
// of contexts actually changed.
func (s *Store) ArchiveContexts(ctx context.Context, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin archive tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	stmt, err := tx.PrepareContext(ctx,
		`UPDATE contexts SET status = ?, updated_at = ? WHERE id = ? AND status <> ?`)
	if err != nil {
		return 0, fmt.Errorf("prepare archive: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	total := 0
	for _, id := range ids {
		res, err := stmt.ExecContext(ctx, contextStatusArchived, now, id, contextStatusArchived)
		if err != nil {
			return 0, fmt.Errorf("archive %s: %w", id, err)
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit archive: %w", err)
	}
	return total, nil
}

// ListContextIDsByJiraStatus returns the distinct ids of contexts that have at
// least one Jira issue whose status matches the given status (case-insensitive).
// The join and filter run in SQLite so callers avoid scanning every context.
func (s *Store) ListContextIDsByJiraStatus(ctx context.Context, status string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT context_id FROM jira_issues WHERE LOWER(status) = LOWER(?)`, status)
	if err != nil {
		return nil, fmt.Errorf("list contexts by jira status: %w", err)
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
		       COALESCE(sn.status, 'not_started') AS snapshot_status,
		       COALESCE(sn.is_stale, 0) AS snapshot_is_stale,
		       COALESCE(latest_log.comment, '') AS snapshot_status_comment
		       ,(SELECT AVG(CASE ji_imp.priority
		             WHEN 'Highest' THEN 5 WHEN 'High' THEN 4 WHEN 'Medium' THEN 3
		             WHEN 'Low' THEN 2 WHEN 'Lowest' THEN 1 END)
		         FROM jira_issues ji_imp WHERE ji_imp.context_id = c.id) AS importance
		FROM contexts c
		LEFT JOIN jira_issues i ON i.context_id = c.id
		LEFT JOIN context_snapshots sn ON sn.context_id = c.id
		LEFT JOIN context_status_log latest_log
		    ON latest_log.id = (
		        SELECT id FROM context_status_log
		        WHERE context_id = c.id
		        ORDER BY created_at DESC
		        LIMIT 1
		    )
		WHERE (? OR c.status = 'active')
		GROUP BY c.id
		ORDER BY c.sort_order ASC NULLS LAST, c.updated_at DESC
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
	var isStaleInt int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT c.id, c.key, c.status, c.org_id, c.created_at, c.updated_at,
		       COALESCE(sn.status, 'not_started'), COALESCE(sn.is_stale, 0),
		       COALESCE(latest_log.comment, '') AS snapshot_status_comment
		FROM contexts c
		LEFT JOIN context_snapshots sn ON sn.context_id = c.id
		LEFT JOIN context_status_log latest_log
		    ON latest_log.id = (
		        SELECT id FROM context_status_log
		        WHERE context_id = c.id
		        ORDER BY created_at DESC
		        LIMIT 1
		    )
		WHERE c.id = ?`,
		id,
	).Scan(&c.ID, &c.Key, &c.Status, &c.OrgID, &c.CreatedAt, &c.UpdatedAt,
		&c.SnapshotStatus, &isStaleInt, &c.SnapshotStatusComment)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting context: %w", err)
	}
	c.SnapshotIsStale = isStaleInt == 1
	return &c, nil
}

// GetContextByKey returns a single context by user-assigned key.
func (s *Store) GetContextByKey(ctx context.Context, key string) (*Context, error) {
	var c Context
	var isStaleInt int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT c.id, c.key, c.status, c.org_id, c.created_at, c.updated_at,
		       COALESCE(sn.status, 'not_started'), COALESCE(sn.is_stale, 0),
		       COALESCE(latest_log.comment, '') AS snapshot_status_comment
		FROM contexts c
		LEFT JOIN context_snapshots sn ON sn.context_id = c.id
		LEFT JOIN context_status_log latest_log
		    ON latest_log.id = (
		        SELECT id FROM context_status_log
		        WHERE context_id = c.id
		        ORDER BY created_at DESC
		        LIMIT 1
		    )
		WHERE c.key = ?`,
		key,
	).Scan(&c.ID, &c.Key, &c.Status, &c.OrgID, &c.CreatedAt, &c.UpdatedAt,
		&c.SnapshotStatus, &isStaleInt, &c.SnapshotStatusComment)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting context by key: %w", err)
	}
	c.SnapshotIsStale = isStaleInt == 1
	return &c, nil
}

// CreateContext inserts a new context and returns it.
// The new context is placed at the top of the list (sort_order = -1 so it sorts before existing rows).
// A subsequent SetContextsOrder call will normalise all values.
func (s *Store) CreateContext(ctx context.Context, key, orgID string) (*Context, error) {
	if orgID == "" {
		return nil, fmt.Errorf("orgID is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	c := &Context{
		ID:             uuid.New().String(),
		Key:            key,
		Status:         contextStatusActive,
		OrgID:          orgID,
		CreatedAt:      now,
		UpdatedAt:      now,
		SnapshotStatus: "not_started",
	}
	// Place the new context first by giving it sort_order = MIN(existing) - 1.
	// Defaults to 0 if there are no existing contexts.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contexts (id, key, status, org_id, created_at, updated_at, sort_order)
		VALUES (?, ?, ?, ?, ?, ?, COALESCE((SELECT MIN(sort_order) - 1 FROM contexts), 0))
	`, c.ID, c.Key, c.Status, c.OrgID, c.CreatedAt, c.UpdatedAt)
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
	res, err := s.db.ExecContext(
		ctx,
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
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT id, id, 'key', key FROM contexts`,
		// Jira issues: title, key, description, and every comment body (item_id = owning issue).
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'jira', summary FROM jira_issues WHERE summary != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'jira_key', jira_key FROM jira_issues WHERE jira_key != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'jira_desc', description FROM jira_issues WHERE description != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT ji.context_id, ji.id, 'jira_comment', jc.body FROM jira_comments jc JOIN jira_issues ji ON ji.id = jc.issue_id WHERE jc.body != ''`,
		// Slack: channel names (item_id = channel) and full thread content (item_id = thread).
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'slack_channel', channel_name FROM slack_channels WHERE channel_name != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'slack_thread', content FROM slack_threads WHERE content != ''`,
		// VCS branches + repo metadata (item_id = branch link row).
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'branch', branch FROM context_vcs_branches WHERE branch != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT cvb.context_id, cvb.id, 'repo', vr.owner || '/' || vr.name FROM context_vcs_branches cvb JOIN vcs_repos vr ON vr.id = cvb.repo_id`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT cvb.context_id, cvb.id, 'repo_desc', vr.description FROM context_vcs_branches cvb JOIN vcs_repos vr ON vr.id = cvb.repo_id WHERE vr.description != ''`,
		// Context texts: name + full body (item_id = text row).
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'text', name FROM context_texts WHERE name != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'text_content', content FROM context_texts WHERE content != ''`,
		// Uploaded files: original name + extracted text (item_id = file row).
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'file', original_name FROM context_uploaded_files WHERE original_name != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT context_id, id, 'file_content', extracted_text FROM context_uploaded_files WHERE extracted_text != ''`,
		// Pull requests: title (item_id = PR) and comment bodies (item_id = owning PR).
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT cvb.context_id, vpr.id, 'pr', vpr.title FROM vcs_pull_requests vpr JOIN context_vcs_branches cvb ON cvb.id = vpr.branch_id WHERE vpr.title != ''`,
		`INSERT INTO context_search_fts (context_id, item_id, source, body) SELECT cvb.context_id, vpr.id, 'pr_comment', vpc.body FROM vcs_pr_comments vpc JOIN vcs_pull_requests vpr ON vpr.id = vpc.pr_id JOIN context_vcs_branches cvb ON cvb.id = vpr.branch_id WHERE vpc.body != ''`,
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
	// All-terms first (precise), then any-term: a multi-token query whose
	// tokens live in different contexts (e.g. several Jira keys pasted at
	// once) must still find each of them instead of returning nothing.
	for _, ftsQuery := range []string{fts5PrefixQuery(query, "AND"), fts5PrefixQuery(query, "OR")} {
		ids, err := s.searchContextIDsFTS(ctx, ftsQuery)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	// ponytail: LIKE fallback for short queries or tokens FTS5 skips (e.g. "pa", "ENG-")
	return s.searchContextIDsLike(ctx, query)
}

func (s *Store) searchContextIDsFTS(ctx context.Context, ftsQuery string) ([]string, error) {
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

// SearchContexts returns ranked cross-org search hits (id, key, org, primary
// summary) for the global search palette. Ranking mirrors SearchContextIDs.
func (s *Store) SearchContexts(ctx context.Context, query string) ([]ContextSearchHit, error) {
	ids, err := s.SearchContextIDs(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.key, COALESCE(c.org_id, ''), COALESCE(o.name, ''), COALESCE(ji.summary, '')
		FROM contexts c
		LEFT JOIN organizations o ON o.id = c.org_id
		LEFT JOIN jira_issues ji ON ji.context_id = c.id AND ji.jira_key = c.key
		WHERE c.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("search contexts detail: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byID := make(map[string]ContextSearchHit, len(ids))
	for rows.Next() {
		var h ContextSearchHit
		if err := rows.Scan(&h.ID, &h.Key, &h.OrgID, &h.OrgName, &h.Summary); err != nil {
			return nil, err
		}
		byID[h.ID] = h
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Preserve the BM25 ranking order from SearchContextIDs.
	hits := make([]ContextSearchHit, 0, len(ids))
	for _, id := range ids {
		if h, ok := byID[id]; ok {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// searchContextIDsLike is a substring fallback used when FTS5 returns nothing.
// Searches context key, jira key/summary, and repo owner+name.
func (s *Store) searchContextIDsLike(ctx context.Context, query string) ([]string, error) {
	q := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT id FROM contexts WHERE key LIKE ?
		UNION
		SELECT DISTINCT context_id FROM jira_issues
		  WHERE jira_key LIKE ? OR summary LIKE ?
		UNION
		SELECT DISTINCT cvb.context_id
		  FROM context_vcs_branches cvb
		  JOIN vcs_repos vr ON vr.id = cvb.repo_id
		  WHERE (vr.owner || '/' || vr.name) LIKE ? OR vr.description LIKE ?
	`, q, q, q, q, q)
	if err != nil {
		return nil, fmt.Errorf("like search: %w", err)
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

// ContextItemMatches groups the IDs of the elements inside a single context that
// matched a search query, bucketed by element type. Empty query yields no match.
type ContextItemMatches struct {
	IssueIDs   []string `json:"issueIds"`
	ChannelIDs []string `json:"channelIds"`
	ThreadIDs  []string `json:"threadIds"`
	TextIDs    []string `json:"textIds"`
	FileIDs    []string `json:"fileIds"`
	BranchIDs  []string `json:"branchIds"`
	PRIDs      []string `json:"prIds"`
}

// SearchContextItems returns the elements within a single context whose indexed
// text matches the query, grouped by element type. It powers the in-context
// filter. An empty query returns an empty (non-nil) match set.
func (s *Store) SearchContextItems(ctx context.Context, contextID, query string) (*ContextItemMatches, error) {
	m := &ContextItemMatches{}
	if strings.TrimSpace(query) == "" {
		return m, nil
	}

	if err := s.RebuildSearchIndex(ctx); err != nil {
		return nil, err
	}

	ftsQuery := fts5PrefixQuery(query, "AND")
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT source, item_id
		FROM context_search_fts
		WHERE context_search_fts MATCH ? AND context_id = ?
	`, ftsQuery, contextID)
	if err != nil {
		return nil, fmt.Errorf("fts item search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seen := map[string]struct{}{}
	for rows.Next() {
		var source, itemID string
		if err := rows.Scan(&source, &itemID); err != nil {
			return nil, err
		}
		m.add(source, itemID, seen)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

// add routes a matched (source, itemID) pair into the right bucket, de-duplicating
// across the multiple FTS sources that can map to the same element.
func (m *ContextItemMatches) add(source, itemID string, seen map[string]struct{}) {
	if itemID == "" {
		return
	}
	var bucket *[]string
	switch source {
	case "jira", "jira_key", "jira_desc", "jira_comment":
		bucket = &m.IssueIDs
	case "slack_channel":
		bucket = &m.ChannelIDs
	case "slack_thread":
		bucket = &m.ThreadIDs
	case "text", "text_content":
		bucket = &m.TextIDs
	case "file", "file_content":
		bucket = &m.FileIDs
	case "branch", "repo", "repo_desc":
		bucket = &m.BranchIDs
	case "pr", "pr_comment":
		bucket = &m.PRIDs
	default: // "key" and anything else: not a filterable element
		return
	}
	if _, ok := seen[bucketKey(bucket, itemID)]; ok {
		return
	}
	seen[bucketKey(bucket, itemID)] = struct{}{}
	*bucket = append(*bucket, itemID)
}

// bucketKey builds a dedup key unique per (bucket, itemID) so the same element
// matched via several sources is only appended once.
func bucketKey(bucket *[]string, itemID string) string {
	return fmt.Sprintf("%p:%s", bucket, itemID)
}

// scanContextRows scans a *sql.Rows result set into []Context.
// Column order must match the SELECT in ListContexts and ListContextsByOrg.
func scanContextRows(rows *sql.Rows) ([]Context, error) {
	var contexts []Context
	for rows.Next() {
		var c Context
		var isStaleInt int
		var importance sql.NullFloat64
		if err := rows.Scan(
			&c.ID, &c.Key, &c.Status, &c.OrgID, &c.CreatedAt, &c.UpdatedAt,
			&c.IssueCount, &c.LastItemAt, &c.UnviewedCount,
			&c.HasJira, &c.HasSlack, &c.HasVCS, &c.HasFiles,
			&c.SnapshotStatus, &isStaleInt, &c.SnapshotStatusComment,
			&importance,
		); err != nil {
			return nil, fmt.Errorf("scanning context: %w", err)
		}
		c.SnapshotIsStale = isStaleInt == 1
		if importance.Valid {
			c.Importance = &importance.Float64
		}
		contexts = append(contexts, c)
	}
	return contexts, rows.Err()
}

// fts5PrefixQuery converts a user query like "batch met" into FTS5 syntax "batch" * AND "met" *.
// Each whitespace-separated token becomes a prefix term.
func fts5PrefixQuery(input, op string) string {
	words := strings.Fields(input)
	parts := make([]string, 0, len(words))
	for _, w := range words {
		escaped := strings.ReplaceAll(w, `"`, `""`)
		parts = append(parts, `"`+escaped+`"*`)
	}
	return strings.Join(parts, " "+op+" ")
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

// ContextIDsForRepo returns the distinct context IDs that track the given repo
// (via context_vcs_branches). Non-nil even when empty.
func (s *Store) ContextIDsForRepo(ctx context.Context, repoID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT context_id FROM context_vcs_branches WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil, fmt.Errorf("context ids for repo: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan context id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
