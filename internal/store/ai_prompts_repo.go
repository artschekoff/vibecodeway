package store

import (
	"context"
	"fmt"
	"time"
)

// knownPlaceholders defines the available {{placeholder}} variables for each prompt.
var knownPlaceholders = map[string][]PromptPlaceholder{
	"event_extraction": {
		{Name: "context_key", Description: "Project/module key (e.g. JH-2210)"},
		{Name: "raw_content", Description: "Raw formatted content from all providers (Jira, Slack, GitHub)"},
		{Name: "user_next_steps", Description: "User-provided next-step hints block (empty if none set)"},
	},
	"snapshot_build": {
		{Name: "context_key", Description: "Project/module key"},
		{Name: "events", Description: "Full event log as JSON array"},
		{Name: "next_steps_short", Description: "User short-term hints (~1 day)"},
		{Name: "next_steps_mid", Description: "User mid-term hints (~1 week)"},
		{Name: "next_steps_long", Description: "User long-term hints (~1 month)"},
	},
	"handoff_packet": {
		{Name: "context_key", Description: "Project/module key"},
		{Name: "events", Description: "Full event log as JSON array"},
	},
	"task_assignment": {
		{Name: "context_key", Description: "Project/module key of the context"},
		{Name: "raw_content", Description: "Formatted Jira issues and activity for this context"},
		{Name: "events", Description: "Full event log in chronological order"},
		{Name: "custom_instructions", Description: "Optional extra instructions section (empty by default)"},
	},
	"task_assignment_refine": {
		{Name: "current_content", Description: "The existing task assignment document to refine"},
		{Name: "user_instruction", Description: "The user's modification instruction"},
		{Name: "raw_content", Description: "Formatted Jira issues and activity (reference)"},
		{Name: "events", Description: "Full event log (reference)"},
	},
}

// ListAIPrompts returns all prompt templates with their placeholder definitions.
func (s *Store) ListAIPrompts(ctx context.Context) ([]AIPrompt, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, label, body, default_body, updated_at
		FROM ai_prompts ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list ai prompts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []AIPrompt
	for rows.Next() {
		var p AIPrompt
		if err := rows.Scan(&p.ID, &p.Label, &p.Body, &p.DefaultBody, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ai prompt: %w", err)
		}
		p.Placeholders = knownPlaceholders[p.ID]
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetAIPrompt returns a single prompt by ID with its placeholders populated.
func (s *Store) GetAIPrompt(ctx context.Context, id string) (*AIPrompt, error) {
	var p AIPrompt
	err := s.db.QueryRowContext(ctx, `
		SELECT id, label, body, default_body, updated_at FROM ai_prompts WHERE id = ?
	`, id).Scan(&p.ID, &p.Label, &p.Body, &p.DefaultBody, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get ai prompt %q: %w", id, err)
	}
	p.Placeholders = knownPlaceholders[p.ID]
	return &p, nil
}

// UpdateAIPrompt replaces the body of an existing prompt.
func (s *Store) UpdateAIPrompt(ctx context.Context, id, body string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE ai_prompts SET body = ?, updated_at = ? WHERE id = ?
	`, body, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update ai prompt %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ai prompt %q not found", id)
	}
	return nil
}

// ResetAIPrompt restores the prompt body to its seeded default.
func (s *Store) ResetAIPrompt(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE ai_prompts SET body = default_body, updated_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("reset ai prompt %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ai prompt %q not found", id)
	}
	return nil
}
