package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetKeybindings returns all keybindings ordered by id.
func (s *Store) GetKeybindings(ctx context.Context) ([]Keybinding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, command, label, description, modifier_keys, key_code, updated_at
		FROM keybindings ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("get keybindings: %w", err)
	}
	defer rows.Close()

	var result []Keybinding
	for rows.Next() {
		var kb Keybinding
		if err := rows.Scan(&kb.ID, &kb.Command, &kb.Label, &kb.Description,
			&kb.ModifierKeys, &kb.KeyCode, &kb.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan keybinding: %w", err)
		}
		result = append(result, kb)
	}
	return result, rows.Err()
}

// GetKeybindingByCommand returns one binding by its command slug.
func (s *Store) GetKeybindingByCommand(ctx context.Context, command string) (*Keybinding, error) {
	var kb Keybinding
	err := s.db.QueryRowContext(ctx, `
		SELECT id, command, label, description, modifier_keys, key_code, updated_at
		FROM keybindings WHERE command = ?
	`, command).Scan(&kb.ID, &kb.Command, &kb.Label, &kb.Description,
		&kb.ModifierKeys, &kb.KeyCode, &kb.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get keybinding by command: %w", err)
	}
	return &kb, nil
}

// UpdateKeybinding saves a new modifier+key combo for an existing command.
// Returns an error when the command does not exist.
func (s *Store) UpdateKeybinding(ctx context.Context, command, modifierKeys, keyCode string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
		UPDATE keybindings
		SET modifier_keys = ?, key_code = ?, updated_at = ?
		WHERE command = ?
	`, modifierKeys, keyCode, now, command)
	if err != nil {
		return fmt.Errorf("update keybinding: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update keybinding rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("update keybinding: command %q not found", command)
	}
	return nil
}
