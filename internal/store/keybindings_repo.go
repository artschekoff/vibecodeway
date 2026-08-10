package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// keybindingSelectColumns lists the columns (in order) shared by every
// keybindings SELECT so GetKeybindings and GetKeybindingByCommand stay in sync.
const keybindingSelectColumns = `id, command, label, description, modifier_keys, key_code,
	object, action, scope, sequence, default_modifier_keys, default_key_code, default_sequence, updated_at`

func scanKeybinding(row interface{ Scan(...any) error }) (Keybinding, error) {
	var kb Keybinding
	err := row.Scan(&kb.ID, &kb.Command, &kb.Label, &kb.Description, &kb.ModifierKeys, &kb.KeyCode,
		&kb.Object, &kb.Action, &kb.Scope, &kb.Sequence, &kb.DefaultModifierKeys, &kb.DefaultKeyCode,
		&kb.DefaultSequence, &kb.UpdatedAt)
	return kb, err
}

// GetKeybindings returns all keybindings ordered by id.
func (s *Store) GetKeybindings(ctx context.Context) ([]Keybinding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+keybindingSelectColumns+`
		FROM keybindings ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("get keybindings: %w", err)
	}
	defer rows.Close()

	var result []Keybinding
	for rows.Next() {
		kb, err := scanKeybinding(rows)
		if err != nil {
			return nil, fmt.Errorf("scan keybinding: %w", err)
		}
		result = append(result, kb)
	}
	return result, rows.Err()
}

// GetKeybindingByCommand returns one binding by its command slug.
func (s *Store) GetKeybindingByCommand(ctx context.Context, command string) (*Keybinding, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+keybindingSelectColumns+`
		FROM keybindings WHERE command = ?
	`, command)
	kb, err := scanKeybinding(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get keybinding by command: %w", err)
	}
	return &kb, nil
}

// UpdateKeybinding saves a new modifier+key+sequence combo for an existing command.
// Returns an error when the command does not exist.
func (s *Store) UpdateKeybinding(ctx context.Context, command, modifierKeys, keyCode, sequence string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
		UPDATE keybindings
		SET modifier_keys = ?, key_code = ?, sequence = ?, updated_at = ?
		WHERE command = ?
	`, modifierKeys, keyCode, sequence, now, command)
	if err != nil {
		return fmt.Errorf("update keybinding: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update keybinding rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResetKeybinding restores a command's chord to its shipped default.
func (s *Store) ResetKeybinding(ctx context.Context, command string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE keybindings
		SET modifier_keys = default_modifier_keys,
		    key_code      = default_key_code,
		    sequence      = default_sequence,
		    updated_at    = ?
		WHERE command = ?`, time.Now().UTC().Format(time.RFC3339), command)
	if err != nil {
		return fmt.Errorf("reset keybinding: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
