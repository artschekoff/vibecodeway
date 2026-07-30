package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListTerminalCommands returns all terminal commands ordered by title.
func (s *Store) ListTerminalCommands(ctx context.Context) ([]TerminalCommand, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, command, send_enter, created_at, updated_at
		FROM terminal_commands ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("list terminal commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TerminalCommand
	for rows.Next() {
		var c TerminalCommand
		var sendEnter int
		if err := rows.Scan(&c.ID, &c.Title, &c.Command, &sendEnter, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan terminal command: %w", err)
		}
		c.SendEnter = sendEnter != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// AddTerminalCommand inserts a new command and returns it.
func (s *Store) AddTerminalCommand(ctx context.Context, title, command string, sendEnter bool) (*TerminalCommand, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	c := TerminalCommand{
		ID:        uuid.New().String(),
		Title:     title,
		Command:   command,
		SendEnter: sendEnter,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO terminal_commands (id, title, command, send_enter, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Title, c.Command, boolToInt(c.SendEnter), c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("add terminal command: %w", err)
	}
	return &c, nil
}

// UpdateTerminalCommand updates a command's title, command text, and send-enter flag.
func (s *Store) UpdateTerminalCommand(ctx context.Context, id, title, command string, sendEnter bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE terminal_commands SET title = ?, command = ?, send_enter = ?, updated_at = ? WHERE id = ?`,
		title, command, boolToInt(sendEnter), now, id)
	if err != nil {
		return fmt.Errorf("update terminal command: %w", err)
	}
	return nil
}

// DeleteTerminalCommand removes a command by id.
func (s *Store) DeleteTerminalCommand(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM terminal_commands WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete terminal command: %w", err)
	}
	return nil
}
