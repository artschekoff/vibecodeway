package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateContextEventHook inserts a hook (enabled) for a context.
func (s *Store) CreateContextEventHook(ctx context.Context, contextID, eventCategory, actionType, scriptCommand string) (*ContextEventHook, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	h := &ContextEventHook{
		ID:            uuid.New().String(),
		ContextID:     contextID,
		EventCategory: eventCategory,
		ActionType:    actionType,
		ScriptCommand: scriptCommand,
		Enabled:       true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_event_hooks
			(id, context_id, event_category, action_type, script_command, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)
	`, h.ID, h.ContextID, h.EventCategory, h.ActionType, h.ScriptCommand, h.CreatedAt, h.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert context event hook: %w", err)
	}
	return h, nil
}

func (s *Store) queryHooks(ctx context.Context, where string, args ...any) ([]ContextEventHook, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, event_category, action_type, script_command, enabled, created_at, updated_at
		FROM context_event_hooks `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("query context event hooks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	hooks := []ContextEventHook{}
	for rows.Next() {
		var h ContextEventHook
		var enabledInt int
		if err := rows.Scan(&h.ID, &h.ContextID, &h.EventCategory, &h.ActionType,
			&h.ScriptCommand, &enabledInt, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan context event hook: %w", err)
		}
		h.Enabled = enabledInt != 0
		hooks = append(hooks, h)
	}
	return hooks, rows.Err()
}

// ListContextEventHooks returns all hooks for a context, newest first.
func (s *Store) ListContextEventHooks(ctx context.Context, contextID string) ([]ContextEventHook, error) {
	return s.queryHooks(ctx, `WHERE context_id = ? ORDER BY created_at DESC`, contextID)
}

// ListEnabledHooks returns enabled hooks for a context and event category.
func (s *Store) ListEnabledHooks(ctx context.Context, contextID, eventCategory string) ([]ContextEventHook, error) {
	return s.queryHooks(ctx, `WHERE context_id = ? AND event_category = ? AND enabled = 1`, contextID, eventCategory)
}

// DeleteContextEventHook removes a hook by id.
func (s *Store) DeleteContextEventHook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM context_event_hooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete context event hook: %w", err)
	}
	return nil
}
