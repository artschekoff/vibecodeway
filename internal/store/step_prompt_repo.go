package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InsertStepPrompt saves the rendered LLM prompt for a reasoning chain step.
// Upserts: replaces any existing row for the same step_id.
func (s *Store) InsertStepPrompt(ctx context.Context, stepID, llmPrompt string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO step_prompts (id, step_id, llm_prompt, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(step_id) DO UPDATE SET llm_prompt = excluded.llm_prompt
	`, uuid.New().String(), stepID, llmPrompt, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert step prompt: %w", err)
	}
	return nil
}

// GetStepPrompt returns the stored LLM prompt for a step, or "" if none.
func (s *Store) GetStepPrompt(ctx context.Context, stepID string) (string, error) {
	var prompt string
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(llm_prompt,'') FROM step_prompts WHERE step_id = ?`, stepID,
	).Scan(&prompt)
	if err != nil {
		return "", nil // not found — return empty
	}
	return prompt, nil
}
