package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateNotification inserts a new notification for a state-change event and
// reports whether a row was actually inserted. If an unread notification of the
// same event_type AND new_value already exists for the entity, it is a no-op
// returning false — that is the same change re-detected on a later sync. A
// different new_value on the same still-unread entity+event_type is a genuinely
// different change and must insert, or its note (and hook dispatch) would be
// silently swallowed by an older, unrelated unread row.
// note is the matched notification rule's text, or "" when no rule matched.
//
// ponytail: eight positional strings is this signature's limit — a ninth turns
// it into a struct argument.
func (s *Store) CreateNotification(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue, note string) (bool, error) {
	var existing string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id FROM notifications WHERE entity_id = ? AND event_type = ? AND new_value = ? AND read_at IS NULL LIMIT 1`,
		entityID, eventType, newValue,
	).Scan(&existing)
	if err == nil {
		return false, nil // already an unread notification of this exact change — skip
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("check existing notification: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO notifications (id, context_id, entity_type, entity_id, event_type, old_value, new_value, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), contextID, entityType, entityID, eventType, oldValue, newValue, note,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, fmt.Errorf("insert notification: %w", err)
	}
	return true, nil
}

// CountUnreadNotifications returns the number of unread notifications for a context.
func (s *Store) CountUnreadNotifications(ctx context.Context, contextID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM notifications WHERE context_id = ? AND read_at IS NULL`,
		contextID,
	).Scan(&n)
	return n, err
}

// MarkNotificationsRead marks all unread notifications for a context as read.
func (s *Store) MarkNotificationsRead(ctx context.Context, contextID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE notifications SET read_at = datetime('now') WHERE context_id = ? AND read_at IS NULL`,
		contextID,
	)
	return err
}

// ListUnreadNotifications returns all unread notifications for a context, newest first.
func (s *Store) ListUnreadNotifications(ctx context.Context, contextID string) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, entity_type, entity_id, event_type, old_value, new_value, note, created_at, read_at
		FROM notifications
		WHERE context_id = ? AND read_at IS NULL
		ORDER BY created_at DESC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list unread notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.ContextID, &n.EntityType, &n.EntityID, &n.EventType,
			&n.OldValue, &n.NewValue, &n.Note, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// ListRecentActivity returns the most recent notifications across all contexts,
// newest first, joined with the context key for display. Includes read and
// unread. limit <= 0 defaults to 100.
func (s *Store) ListRecentActivity(ctx context.Context, limit int) ([]ActivityFeedItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.id, n.context_id, c.key, n.entity_type, n.event_type, n.old_value, n.new_value, n.note, n.created_at
		FROM notifications n
		JOIN contexts c ON c.id = n.context_id
		ORDER BY n.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent activity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []ActivityFeedItem{}
	for rows.Next() {
		var it ActivityFeedItem
		if err := rows.Scan(&it.ID, &it.ContextID, &it.ContextKey, &it.EntityType,
			&it.EventType, &it.OldValue, &it.NewValue, &it.Note, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan activity item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
