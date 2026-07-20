package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateNotification inserts a new notification for a state-change event.
// If an unread notification of the same event_type already exists for the entity,
// it is a no-op (prevents duplicate alerts on repeated syncs).
func (s *Store) CreateNotification(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error {
	var existing string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM notifications WHERE entity_id = ? AND event_type = ? AND read_at IS NULL LIMIT 1`,
		entityID, eventType,
	).Scan(&existing)
	if err == nil {
		return nil // already an unread notification of this type — skip
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check existing notification: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO notifications (id, context_id, entity_type, entity_id, event_type, old_value, new_value, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, uuid.New().String(), contextID, entityType, entityID, eventType, oldValue, newValue,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// CountUnreadNotifications returns the number of unread notifications for a context.
func (s *Store) CountUnreadNotifications(ctx context.Context, contextID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE context_id = ? AND read_at IS NULL`,
		contextID,
	).Scan(&n)
	return n, err
}

// MarkNotificationsRead marks all unread notifications for a context as read.
func (s *Store) MarkNotificationsRead(ctx context.Context, contextID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET read_at = datetime('now') WHERE context_id = ? AND read_at IS NULL`,
		contextID,
	)
	return err
}

// ListUnreadNotifications returns all unread notifications for a context, newest first.
func (s *Store) ListUnreadNotifications(ctx context.Context, contextID string) ([]Notification, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, entity_type, entity_id, event_type, old_value, new_value, created_at, read_at
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
			&n.OldValue, &n.NewValue, &n.CreatedAt, &n.ReadAt); err != nil {
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
		SELECT n.id, n.context_id, c.key, n.entity_type, n.event_type, n.old_value, n.new_value, n.created_at
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
			&it.EventType, &it.OldValue, &it.NewValue, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan activity item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
