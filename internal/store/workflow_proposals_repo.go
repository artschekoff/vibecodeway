package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SetAIProposal writes the latest AI-computed workflow values to the contexts row
// and creates a new entry in context_workflow_proposals for the audit log.
// Both writes are wrapped in a transaction — if the proposal INSERT fails, the
// contexts row update is rolled back so the two tables stay in sync.
func (s *Store) SetAIProposal(ctx context.Context, contextID, status, comment, nextStep, prevUserNextStep string) (*WorkflowProposal, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var commentPtr, nextStepPtr, prevPtr *string
	if comment != "" {
		commentPtr = &comment
	}
	if nextStep != "" {
		nextStepPtr = &nextStep
	}
	if prevUserNextStep != "" {
		prevPtr = &prevUserNextStep
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("set ai proposal begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE contexts
		SET ai_proposed_status    = ?,
		    ai_proposed_comment   = ?,
		    ai_proposed_next_step = ?,
		    ai_proposed_at        = ?
		WHERE id = ?
	`, status, commentPtr, nextStepPtr, now, contextID)
	if err != nil {
		return nil, fmt.Errorf("set ai proposal on context: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}

	p := &WorkflowProposal{
		ID:               uuid.New().String(),
		ContextID:        contextID,
		ProposedStatus:   status,
		ProposedComment:  commentPtr,
		ProposedNextStep: nextStepPtr,
		PrevUserNextStep: prevPtr,
		UpdatedAt:        now,
		CreatedAt:        now,
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO context_workflow_proposals
		    (id, context_id, proposed_status, proposed_comment, proposed_next_step, prev_user_next_step, updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.ContextID, p.ProposedStatus, p.ProposedComment, p.ProposedNextStep, p.PrevUserNextStep, p.UpdatedAt, p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert workflow proposal: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("set ai proposal commit: %w", err)
	}
	return p, nil
}

// SetUserOverride stores the user's overrides for status, comment, and next-step.
// Pass empty string for any field that should be cleared (reverts to AI proposal).
func (s *Store) SetUserOverride(ctx context.Context, contextID, status, comment, nextStep string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	toPtr := func(v string) *string {
		if v == "" {
			return nil
		}
		return &v
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE contexts
		SET user_status_override     = ?,
		    user_comment_override    = ?,
		    user_next_step_override  = ?,
		    user_override_updated_at = ?
		WHERE id = ?
	`, toPtr(status), toPtr(comment), toPtr(nextStep), now, contextID)
	if err != nil {
		return fmt.Errorf("set user override: %w", err)
	}

	// Mark the latest unresolved proposal as resolved with the user's choices.
	// NOTE: SQLite does not support ORDER BY / LIMIT in UPDATE directly,
	// so use a subquery to identify the target row.
	// Zero rows affected is acceptable — it means no unresolved proposal exists.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE context_workflow_proposals
		SET user_status_override    = ?,
		    user_comment_override   = ?,
		    user_next_step_override = ?,
		    resolved_at             = ?,
		    updated_at              = ?
		WHERE id = (
		    SELECT id FROM context_workflow_proposals
		    WHERE context_id = ? AND resolved_at IS NULL
		    ORDER BY created_at DESC
		    LIMIT 1
		)
	`, toPtr(status), toPtr(comment), toPtr(nextStep), now, now, contextID); err != nil {
		return fmt.Errorf("mark proposal resolved: %w", err)
	}

	return nil
}

// ListWorkflowProposals returns all AI proposals for a context, newest first.
func (s *Store) ListWorkflowProposals(ctx context.Context, contextID string) ([]WorkflowProposal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, proposed_status, proposed_comment, proposed_next_step,
		       user_status_override, user_comment_override, user_next_step_override,
		       prev_user_next_step, resolved_at, updated_at, created_at
		FROM context_workflow_proposals
		WHERE context_id = ? ORDER BY created_at DESC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list workflow proposals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []WorkflowProposal
	for rows.Next() {
		var p WorkflowProposal
		if err := rows.Scan(&p.ID, &p.ContextID,
			&p.ProposedStatus, &p.ProposedComment, &p.ProposedNextStep,
			&p.UserStatusOverride, &p.UserCommentOverride, &p.UserNextStepOverride,
			&p.PrevUserNextStep, &p.ResolvedAt, &p.UpdatedAt, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workflow proposal: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
