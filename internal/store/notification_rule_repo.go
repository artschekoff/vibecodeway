package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateNotificationRule stores a new watch and returns it as saved. matchValue
// may be "" to match any new value; note is the text the user will be shown and
// callers are expected to reject a blank one before getting here.
func (s *Store) CreateNotificationRule(ctx context.Context, contextID, eventType, matchValue, note string) (NotificationRule, error) {
	r := NotificationRule{
		ID:         uuid.New().String(),
		ContextID:  contextID,
		EventType:  eventType,
		MatchValue: matchValue,
		Note:       note,
		Enabled:    true,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO notification_rules (id, context_id, event_type, match_value, note, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)
	`, r.ID, r.ContextID, r.EventType, r.MatchValue, r.Note, r.CreatedAt)
	if err != nil {
		return NotificationRule{}, fmt.Errorf("insert notification rule: %w", err)
	}
	return r, nil
}

// ListNotificationRules returns a context's rules in creation order. Order is
// part of the contract: NoteForEvent takes the first match, so the user's
// earliest rule wins when two of them match one event.
//
// Ordered by rowid, not created_at: created_at is RFC3339 with one-second
// precision, so two rules added in the same second would tie and fall through
// to a random UUID. rowid is insertion order by construction.
func (s *Store) ListNotificationRules(ctx context.Context, contextID string) ([]NotificationRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, event_type, match_value, note, enabled, created_at
		FROM notification_rules
		WHERE context_id = ?
		ORDER BY rowid
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list notification rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	rules := []NotificationRule{}
	for rows.Next() {
		var r NotificationRule
		if err := rows.Scan(&r.ID, &r.ContextID, &r.EventType, &r.MatchValue,
			&r.Note, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
