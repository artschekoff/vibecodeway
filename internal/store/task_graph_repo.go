package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// TaskNode is a cached task-graph node (see migration 130).
type TaskNode struct {
	Key          string `json:"key"`
	Summary      string `json:"summary"`
	Status       string `json:"status"`
	IssueType    string `json:"issueType"`
	Priority     string `json:"priority"`
	PriorityRank int    `json:"priorityRank"`
	Assignee     string `json:"assignee"`
	ParentKey    string `json:"parentKey"`
	Mine         bool   `json:"mine"`
	Terminal     bool   `json:"terminal"`
	FetchedAt    string `json:"fetchedAt"`
	DetailAt     string `json:"detailAt"`
}

// TaskEdge is a directed relation. Kind: parent (src's parent is dst),
// blocks (src blocks dst), relates.
type TaskEdge struct {
	Src  string `json:"src"`
	Dst  string `json:"dst"`
	Kind string `json:"kind"`
}

// ResetTaskMine clears the mine flag for every node of provider; the refresh
// re-marks the current assignments so reassigned issues drop out.
func (s *Store) ResetTaskMine(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_nodes SET mine = 0 WHERE provider = ?`, provider)
	return err
}

// UpsertTaskNodes inserts or updates nodes. Empty incoming fields never erase
// known values (a link ref only carries key/status/summary), and mine is
// sticky within a refresh (reset first via ResetTaskMine).
func (s *Store) UpsertTaskNodes(ctx context.Context, provider string, nodes []TaskNode) error {
	if len(nodes) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, n := range nodes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_nodes (provider, key, summary, status, issue_type, priority, priority_rank, assignee, parent_key, mine, terminal, fetched_at, detail_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(provider, key) DO UPDATE SET
				summary       = COALESCE(NULLIF(excluded.summary, ''), task_nodes.summary),
				status        = COALESCE(NULLIF(excluded.status, ''), task_nodes.status),
				issue_type    = COALESCE(NULLIF(excluded.issue_type, ''), task_nodes.issue_type),
				priority      = COALESCE(NULLIF(excluded.priority, ''), task_nodes.priority),
				priority_rank = CASE WHEN excluded.priority <> '' THEN excluded.priority_rank ELSE task_nodes.priority_rank END,
				assignee      = COALESCE(NULLIF(excluded.assignee, ''), task_nodes.assignee),
				parent_key    = COALESCE(NULLIF(excluded.parent_key, ''), task_nodes.parent_key),
				mine          = MAX(task_nodes.mine, excluded.mine),
				terminal      = CASE WHEN excluded.status <> '' THEN excluded.terminal ELSE task_nodes.terminal END,
				fetched_at    = COALESCE(NULLIF(excluded.fetched_at, ''), task_nodes.fetched_at),
				detail_at     = COALESCE(NULLIF(excluded.detail_at, ''), task_nodes.detail_at)`,
			provider, n.Key, n.Summary, n.Status, n.IssueType, n.Priority, n.PriorityRank, n.Assignee, n.ParentKey,
			boolToInt(n.Mine), boolToInt(n.Terminal), n.FetchedAt, n.DetailAt); err != nil {
			return fmt.Errorf("upsert task node %s: %w", n.Key, err)
		}
	}
	return tx.Commit()
}

// ReplaceTaskEdges swaps the edges produced by owner's detail fetch.
func (s *Store) ReplaceTaskEdges(ctx context.Context, provider, owner string, edges []TaskEdge) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_edges WHERE provider = ? AND owner = ?`, provider, owner); err != nil {
		return err
	}
	for _, e := range edges {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO task_edges (provider, owner, src, dst, kind) VALUES (?, ?, ?, ?, ?)`,
			provider, owner, e.Src, e.Dst, e.Kind); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListTaskGraph returns every node and (deduplicated) edge for provider.
func (s *Store) ListTaskGraph(ctx context.Context, provider string) ([]TaskNode, []TaskEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key, summary, status, issue_type, priority, priority_rank, assignee, parent_key, mine, terminal, fetched_at, detail_at
		FROM task_nodes WHERE provider = ? ORDER BY key`, provider)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	var nodes []TaskNode
	for rows.Next() {
		var n TaskNode
		var mine, terminal int
		if err := rows.Scan(&n.Key, &n.Summary, &n.Status, &n.IssueType, &n.Priority, &n.PriorityRank, &n.Assignee, &n.ParentKey, &mine, &terminal, &n.FetchedAt, &n.DetailAt); err != nil {
			return nil, nil, err
		}
		n.Mine, n.Terminal = mine != 0, terminal != 0
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	erows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT src, dst, kind FROM task_edges WHERE provider = ? ORDER BY src, dst, kind`, provider)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = erows.Close() }()
	var edges []TaskEdge
	for erows.Next() {
		var e TaskEdge
		if err := erows.Scan(&e.Src, &e.Dst, &e.Kind); err != nil {
			return nil, nil, err
		}
		edges = append(edges, e)
	}
	return nodes, edges, erows.Err()
}

// PruneTaskNodes deletes non-mine nodes no edge touches (orphans left by
// reassignment or removed links) and their stale edges.
func (s *Store) PruneTaskNodes(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM task_nodes WHERE provider = ? AND mine = 0 AND key NOT IN (
			SELECT src FROM task_edges WHERE provider = ?
			UNION SELECT dst FROM task_edges WHERE provider = ?)`, provider, provider, provider)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM task_edges WHERE provider = ? AND owner NOT IN (SELECT key FROM task_nodes WHERE provider = ?)`,
		provider, provider)
	return err
}

// ContextRef is an active in-app context linked to a tracker key.
type ContextRef struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// FindActiveContextsByJiraKeys maps each Jira key to one active context
// (deterministic MIN(context_id)), skipping keys without one.
func (s *Store) FindActiveContextsByJiraKeys(ctx context.Context, keys []string) (map[string]ContextRef, error) {
	out := map[string]ContextRef{}
	if len(keys) == 0 {
		return out, nil
	}
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ji.jira_key, c.id, c.key
		FROM (SELECT jira_key, MIN(context_id) AS context_id FROM jira_issues
		      WHERE jira_key IN (`+strings.Repeat(",?", len(keys))[1:]+`) GROUP BY jira_key) ji
		JOIN contexts c ON c.id = ji.context_id AND c.status = 'active'`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var jk string
		var ref ContextRef
		if err := rows.Scan(&jk, &ref.ID, &ref.Key); err != nil {
			return nil, err
		}
		out[jk] = ref
	}
	return out, rows.Err()
}

var _ = sql.ErrNoRows
