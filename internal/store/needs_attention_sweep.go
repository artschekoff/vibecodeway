package store

import (
	"context"
	"fmt"
)

// ClearStaleNeedsAttention takes the needs_attention flag back off every context
// where there is nothing left to react to, and returns how many it cleared.
//
// FlagNeedsAttention is called on every incoming sync notification but nothing
// ever cleared it, so the flag accumulated forever and the "Needs Attention"
// panel filled up with contexts the user had already dealt with. A context is
// stale when it is archived (the user closed it out) or when every notification
// on it has been read (the user has seen everything that triggered the flag).
//
// Cleared contexts fall back to the status recorded immediately before the flag
// went on — the status log's prev_id chain — or to not_started when no earlier
// status exists.
//
// ponytail: one sweep over the flagged rows on each context-list load; add a
// scheduler only if the row count ever makes that measurably slow.
func (s *Store) ClearStaleNeedsAttention(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE context_snapshots
		SET status = COALESCE(
		        (SELECT prev.status_slug
		         FROM context_status_log flag
		         JOIN context_status_log prev ON prev.id = flag.prev_id
		         WHERE flag.context_id = context_snapshots.context_id
		           AND flag.status_slug = 'needs_attention'
		         ORDER BY flag.created_at DESC, flag.rowid DESC
		         LIMIT 1),
		        'not_started')
		WHERE status = 'needs_attention'
		  -- Only undo what the sync layer flagged itself; a needs_attention the
		  -- user or the AI chose deliberately is a judgement, not noise.
		  -- No log entry at all means an orphaned legacy flag: it renders as a
		  -- blank tile with no reason, so there is nothing worth keeping.
		  AND COALESCE((SELECT l.source FROM context_status_log l
		       WHERE l.context_id = context_snapshots.context_id
		       ORDER BY l.created_at DESC, l.rowid DESC
		       LIMIT 1), 'system') = 'system'
		  AND (
		        (SELECT c.status FROM contexts c WHERE c.id = context_snapshots.context_id) <> 'active'
		     OR NOT EXISTS (SELECT 1 FROM notifications n
		                    WHERE n.context_id = context_snapshots.context_id
		                      AND n.read_at IS NULL)
		  )
	`)
	if err != nil {
		return 0, fmt.Errorf("clear stale needs_attention: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("clear stale needs_attention rows: %w", err)
	}
	return int(n), nil
}
