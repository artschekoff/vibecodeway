package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateGlobalEventHook inserts an enabled global hook.
func (s *Store) CreateGlobalEventHook(ctx context.Context, eventCategory, actionType, scriptCommand string) (*GlobalEventHook, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	h := &GlobalEventHook{
		ID:            uuid.New().String(),
		EventCategory: eventCategory,
		ActionType:    actionType,
		ScriptCommand: scriptCommand,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO global_event_hooks
			(id, event_category, action_type, script_command, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)
	`, h.ID, h.EventCategory, h.ActionType, h.ScriptCommand, h.CreatedAt, h.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert global event hook: %w", err)
	}
	return h, nil
}

func (s *Store) queryGlobalHooks(ctx context.Context, where string, args ...any) ([]GlobalEventHook, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_category, action_type, script_command, enabled, created_at, updated_at
		FROM global_event_hooks `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("query global event hooks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	hooks := []GlobalEventHook{}
	for rows.Next() {
		var h GlobalEventHook
		var enabledInt int
		if err := rows.Scan(&h.ID, &h.EventCategory, &h.ActionType, &h.ScriptCommand,
			&enabledInt, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan global event hook: %w", err)
		}
		h.Enabled = enabledInt != 0
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

// ListGlobalEventHooks returns all global hooks, newest first.
func (s *Store) ListGlobalEventHooks(ctx context.Context) ([]GlobalEventHook, error) {
	return s.queryGlobalHooks(ctx, `ORDER BY created_at DESC`)
}

// ListEnabledGlobalHooks returns enabled global hooks for an event category.
func (s *Store) ListEnabledGlobalHooks(ctx context.Context, eventCategory string) ([]GlobalEventHook, error) {
	return s.queryGlobalHooks(ctx, `WHERE event_category = ? AND enabled = 1`, eventCategory)
}

// DeleteGlobalEventHook removes a global hook by id.
func (s *Store) DeleteGlobalEventHook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM global_event_hooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete global event hook: %w", err)
	}
	return nil
}
