package store

import (
	"context"
	"fmt"
)

// AIUsage is one LLM call's token bill, recorded at the client dispatch level so
// every feature (Ask AI, Solve AI, analysis, todo generation) is counted.
type AIUsage struct {
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	OccurredAt       string // RFC3339 UTC
}

// InsertAIUsage appends one usage row. Best-effort callers must not fail the
// LLM call on an insert error.
func (s *Store) InsertAIUsage(ctx context.Context, u AIUsage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_usage (provider, model, prompt_tokens, completion_tokens, total_tokens, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.Provider, u.Model, u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert ai usage: %w", err)
	}
	return nil
}

// ListAIUsageSince returns usage rows with occurred_at >= since (RFC3339 UTC),
// oldest first.
func (s *Store) ListAIUsageSince(ctx context.Context, since string) ([]AIUsage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT provider, model, prompt_tokens, completion_tokens, total_tokens, occurred_at
		FROM ai_usage
		WHERE occurred_at >= ?
		ORDER BY occurred_at ASC`, since)
	if err != nil {
		return nil, fmt.Errorf("list ai usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []AIUsage
	for rows.Next() {
		var u AIUsage
		if err := rows.Scan(&u.Provider, &u.Model, &u.PromptTokens, &u.CompletionTokens, &u.TotalTokens, &u.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan ai usage: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
