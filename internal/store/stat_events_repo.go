package store

import (
	"context"
	"fmt"
)

// StatEvent is a single raw activity event feeding the analytics dashboard:
// a PR merge or an authored comment.
type StatEvent struct {
	Kind       string
	ExternalID string
	Org        string
	Repo       string
	JiraKey    string
	OccurredAt string
}

// InsertStatEvent records one stat event. Refreshes are idempotent: seeing
// the same (kind, external_id) again is a no-op, not an error or duplicate.
func (s *Store) InsertStatEvent(ctx context.Context, e StatEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO stat_events (kind, external_id, org, repo, jira_key, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		e.Kind, e.ExternalID, e.Org, e.Repo, e.JiraKey, e.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert stat event %s/%s: %w", e.Kind, e.ExternalID, err)
	}
	return nil
}

// ListStatEventsSince returns stat events with occurred_at >= since
// (RFC3339 UTC string), oldest first.
func (s *Store) ListStatEventsSince(ctx context.Context, since string) ([]StatEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind, external_id, org, repo, jira_key, occurred_at
		FROM stat_events
		WHERE occurred_at >= ?
		ORDER BY occurred_at ASC`, since)
	if err != nil {
		return nil, fmt.Errorf("list stat events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []StatEvent
	for rows.Next() {
		var e StatEvent
		if err := rows.Scan(&e.Kind, &e.ExternalID, &e.Org, &e.Repo, &e.JiraKey, &e.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan stat event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
