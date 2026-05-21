package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	ChainEntityPersonAssignment = "person_assignment"
	ChainEntityContext          = "context"
)

// AppendChainStep appends a new step to the reasoning chain for (entityType, entityID).
// step_index is set to max(existing)+1 atomically.
func (s *Store) AppendChainStep(ctx context.Context, step ReasoningChainStep) (*ReasoningChainStep, error) {
	var maxIdx int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(step_index), -1) FROM reasoning_chain_steps WHERE entity_type = ? AND entity_id = ?`,
		step.EntityType, step.EntityID,
	).Scan(&maxIdx)
	step.StepIndex = maxIdx + 1
	step.ID = uuid.New().String()
	if step.CreatedAt == "" {
		step.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	isCurrent := 0
	if step.IsCurrent {
		isCurrent = 1
	}

	var err error
	if isCurrent == 0 {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO reasoning_chain_steps
			  (id, entity_type, entity_id, step_index, source_type, message_id,
			   message_content, user_instruction, input_state, output_state,
			   compressed_summary, events_snapshot, prompt_version,
			   quality_score, quality_notes, is_current, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, step.ID, step.EntityType, step.EntityID, step.StepIndex, step.SourceType,
			dbNullStr(step.MessageID), step.MessageContent, step.UserInstruction,
			step.InputState, step.OutputState,
			step.CompressedSummary, step.EventsSnapshot, step.PromptVersion,
			step.QualityScore, step.QualityNotes, 0, step.CreatedAt)
	} else {
		// Atomically clear other is_current flags then insert this step as current.
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, fmt.Errorf("begin append step tx: %w", txErr)
		}
		defer func() { _ = tx.Rollback() }()
		if _, txErr = tx.ExecContext(ctx,
			`UPDATE reasoning_chain_steps SET is_current = 0 WHERE entity_type = ? AND entity_id = ?`,
			step.EntityType, step.EntityID,
		); txErr != nil {
			return nil, fmt.Errorf("clear current flags: %w", txErr)
		}
		if _, txErr = tx.ExecContext(ctx, `
			INSERT INTO reasoning_chain_steps
			  (id, entity_type, entity_id, step_index, source_type, message_id,
			   message_content, user_instruction, input_state, output_state,
			   compressed_summary, events_snapshot, prompt_version,
			   quality_score, quality_notes, is_current, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, step.ID, step.EntityType, step.EntityID, step.StepIndex, step.SourceType,
			dbNullStr(step.MessageID), step.MessageContent, step.UserInstruction,
			step.InputState, step.OutputState,
			step.CompressedSummary, step.EventsSnapshot, step.PromptVersion,
			step.QualityScore, step.QualityNotes, 1, step.CreatedAt,
		); txErr != nil {
			return nil, fmt.Errorf("insert chain step: %w", txErr)
		}
		if txErr = tx.Commit(); txErr != nil {
			return nil, fmt.Errorf("commit append step: %w", txErr)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("append chain step: %w", err)
	}
	if step.LLMPrompt != "" {
		if promptErr := s.InsertStepPrompt(ctx, step.ID, step.LLMPrompt); promptErr != nil {
			return nil, fmt.Errorf("save step prompt: %w", promptErr)
		}
	}
	return &step, nil
}

// ListChainSteps returns all steps for (entityType, entityID) ordered by step_index ASC.
func (s *Store) ListChainSteps(ctx context.Context, entityType, entityID string) ([]ReasoningChainStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.entity_type, r.entity_id, r.step_index, r.source_type,
		       COALESCE(r.message_id,''), r.message_content, COALESCE(r.user_instruction,''),
		       r.input_state, r.output_state,
		       COALESCE(r.compressed_summary,''), COALESCE(r.events_snapshot,''),
		       COALESCE(r.prompt_version,'v1'), r.quality_score, COALESCE(r.quality_notes,''),
		       r.is_current, r.created_at,
		       COALESCE(sp.llm_prompt,'')
		FROM reasoning_chain_steps r
		LEFT JOIN step_prompts sp ON sp.step_id = r.id
		WHERE r.entity_type = ? AND r.entity_id = ?
		ORDER BY r.step_index ASC
	`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list chain steps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var steps []ReasoningChainStep
	for rows.Next() {
		var st ReasoningChainStep
		var isCurrent int
		var qualityScore sql.NullFloat64
		if err := rows.Scan(
			&st.ID, &st.EntityType, &st.EntityID, &st.StepIndex, &st.SourceType,
			&st.MessageID, &st.MessageContent, &st.UserInstruction,
			&st.InputState, &st.OutputState,
			&st.CompressedSummary, &st.EventsSnapshot,
			&st.PromptVersion, &qualityScore, &st.QualityNotes,
			&isCurrent, &st.CreatedAt, &st.LLMPrompt,
		); err != nil {
			return nil, fmt.Errorf("scan chain step: %w", err)
		}
		st.IsCurrent = isCurrent != 0
		if qualityScore.Valid {
			st.QualityScore = &qualityScore.Float64
		}
		steps = append(steps, st)
	}
	return steps, rows.Err()
}

// SetChainCurrentStep marks stepID as is_current=1; clears all others for (entityType, entityID).
// Returns the output_state of the newly current step.
func (s *Store) SetChainCurrentStep(ctx context.Context, entityType, entityID, stepID string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin set current: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE reasoning_chain_steps SET is_current = 0 WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID,
	); err != nil {
		return "", fmt.Errorf("clear current flags: %w", err)
	}

	// RETURNING requires SQLite >= 3.35; if older, do a separate SELECT after UPDATE.
	var outputState string
	if err := tx.QueryRowContext(ctx,
		`UPDATE reasoning_chain_steps SET is_current = 1 WHERE id = ? RETURNING output_state`,
		stepID,
	).Scan(&outputState); err != nil {
		return "", fmt.Errorf("set current step: %w", err)
	}

	return outputState, tx.Commit()
}

// TruncateChainFrom deletes all steps with step_index >= fromStepIndex for (entityType, entityID).
func (s *Store) TruncateChainFrom(ctx context.Context, entityType, entityID string, fromStepIndex int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM reasoning_chain_steps WHERE entity_type = ? AND entity_id = ? AND step_index >= ?`,
		entityType, entityID, fromStepIndex,
	)
	return err
}

// UpdateChainStepQuality writes the quality_score and quality_notes fields for an existing step.
func (s *Store) UpdateChainStepQuality(ctx context.Context, stepID string, score float64, notes string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reasoning_chain_steps SET quality_score = ?, quality_notes = ? WHERE id = ?`,
		score, notes, stepID,
	)
	if err != nil {
		return fmt.Errorf("update chain step quality: %w", err)
	}
	return nil
}

// dbNullStr returns nil for empty strings so they are stored as SQL NULL.
func dbNullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
