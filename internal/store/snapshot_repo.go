package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetSnapshot returns the current snapshot for a context, or nil if none exists yet.
func (s *Store) GetSnapshot(ctx context.Context, contextID string) (*ContextSnapshot, error) {
	var sn ContextSnapshot
	var (
		isStaleInt, overriddenInt   int
		estimatedHours              sql.NullFloat64
		estimatedCompletionDate     sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT context_id,
		       current_step_desc, current_step_confidence, current_step_from, current_step_overridden,
		       section_operational, section_narrative, section_engineering,
		       status,
		       ai_next_steps_short, ai_next_steps_mid, ai_next_steps_long,
		       user_next_steps_short, user_next_steps_mid, user_next_steps_long,
		       last_analyzed_session_id, analyzed_at, is_stale, version,
		       estimated_hours, estimated_completion_date,
		       estimation_confidence, estimation_basis
		FROM context_snapshots WHERE context_id = ?
	`, contextID).Scan(
		&sn.ContextID,
		&sn.CurrentStepDesc, &sn.CurrentStepConfidence, &sn.CurrentStepFrom, &overriddenInt,
		&sn.SectionOperational, &sn.SectionNarrative, &sn.SectionEngineering,
		&sn.Status,
		&sn.AINextStepsShort, &sn.AINextStepsMid, &sn.AINextStepsLong,
		&sn.UserNextStepsShort, &sn.UserNextStepsMid, &sn.UserNextStepsLong,
		&sn.LastAnalyzedSessionID, &sn.AnalyzedAt, &isStaleInt, &sn.Version,
		&estimatedHours, &estimatedCompletionDate,
		&sn.EstimationConfidence, &sn.EstimationBasis,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	sn.IsStale = isStaleInt == 1
	sn.CurrentStepOverridden = overriddenInt == 1
	if estimatedHours.Valid {
		v := estimatedHours.Float64
		sn.EstimatedHours = &v
	}
	if estimatedCompletionDate.Valid {
		sn.EstimatedCompletionDate = &estimatedCompletionDate.String
	}
	// Attach latest status log entry
	latest, err := s.GetLatestStatusLogEntry(ctx, contextID)
	if err != nil {
		return nil, err
	}
	sn.LatestStatusLogEntry = latest
	return &sn, nil
}

// UpsertSnapshot creates or fully replaces the snapshot for a context.
func (s *Store) UpsertSnapshot(ctx context.Context, sn *ContextSnapshot) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if sn.AnalyzedAt == nil {
		sn.AnalyzedAt = &now
	}
	var isStaleInt, overriddenInt int
	if sn.IsStale {
		isStaleInt = 1
	}
	if sn.CurrentStepOverridden {
		overriddenInt = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_snapshots
		  (context_id, current_step_desc, current_step_confidence, current_step_from, current_step_overridden,
		   section_operational, section_narrative, section_engineering,
		   status,
		   ai_next_steps_short, ai_next_steps_mid, ai_next_steps_long,
		   user_next_steps_short, user_next_steps_mid, user_next_steps_long,
		   last_analyzed_session_id, analyzed_at, is_stale, version,
		   estimated_hours, estimated_completion_date,
		   estimation_confidence, estimation_basis)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(context_id) DO UPDATE SET
		  current_step_desc         = excluded.current_step_desc,
		  current_step_confidence   = excluded.current_step_confidence,
		  current_step_from         = excluded.current_step_from,
		  current_step_overridden   = excluded.current_step_overridden,
		  section_operational       = excluded.section_operational,
		  section_narrative         = excluded.section_narrative,
		  section_engineering       = excluded.section_engineering,
		  status                    = excluded.status,
		  ai_next_steps_short       = excluded.ai_next_steps_short,
		  ai_next_steps_mid         = excluded.ai_next_steps_mid,
		  ai_next_steps_long        = excluded.ai_next_steps_long,
		  last_analyzed_session_id  = excluded.last_analyzed_session_id,
		  analyzed_at               = excluded.analyzed_at,
		  is_stale                  = excluded.is_stale,
		  version                   = excluded.version,
		  estimated_hours           = excluded.estimated_hours,
		  estimated_completion_date = excluded.estimated_completion_date,
		  estimation_confidence     = excluded.estimation_confidence,
		  estimation_basis          = excluded.estimation_basis
	`,
		sn.ContextID,
		sn.CurrentStepDesc, sn.CurrentStepConfidence, sn.CurrentStepFrom, overriddenInt,
		sn.SectionOperational, sn.SectionNarrative, sn.SectionEngineering,
		sn.Status,
		sn.AINextStepsShort, sn.AINextStepsMid, sn.AINextStepsLong,
		sn.UserNextStepsShort, sn.UserNextStepsMid, sn.UserNextStepsLong,
		sn.LastAnalyzedSessionID, sn.AnalyzedAt, isStaleInt, sn.Version,
		sn.EstimatedHours, sn.EstimatedCompletionDate,
		sn.EstimationConfidence, sn.EstimationBasis,
	)
	if err != nil {
		return fmt.Errorf("upsert snapshot: %w", err)
	}
	return nil
}

// UpdateUserNextSteps saves only the user-written next-step notes (no re-analysis needed).
func (s *Store) UpdateUserNextSteps(ctx context.Context, contextID, short, mid, long string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_snapshots (context_id, user_next_steps_short, user_next_steps_mid, user_next_steps_long)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(context_id) DO UPDATE SET
		  user_next_steps_short = excluded.user_next_steps_short,
		  user_next_steps_mid   = excluded.user_next_steps_mid,
		  user_next_steps_long  = excluded.user_next_steps_long
	`, contextID, short, mid, long)
	if err != nil {
		return fmt.Errorf("update user next steps: %w", err)
	}
	return nil
}

// MarkSnapshotStale sets is_stale = 1 so the UI can prompt re-analysis.
// Creates an empty snapshot row if none exists yet.
func (s *Store) MarkSnapshotStale(ctx context.Context, contextID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_snapshots (context_id, is_stale)
		VALUES (?, 1)
		ON CONFLICT(context_id) DO UPDATE SET is_stale = 1
	`, contextID)
	if err != nil {
		return fmt.Errorf("mark snapshot stale: %w", err)
	}
	return nil
}

// OverrideCurrentStep saves a user correction for the current step and marks the snapshot stale.
func (s *Store) OverrideCurrentStep(ctx context.Context, contextID, description string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_snapshots (context_id, current_step_desc, current_step_confidence, current_step_overridden, is_stale)
		VALUES (?, ?, 'high', 1, 1)
		ON CONFLICT(context_id) DO UPDATE SET
		  current_step_desc       = excluded.current_step_desc,
		  current_step_confidence = 'high',
		  current_step_overridden = 1,
		  is_stale                = 1
	`, contextID, description)
	if err != nil {
		return fmt.Errorf("override current step: %w", err)
	}
	return nil
}

// SetSnapshotStatus updates only the status field on a context snapshot.
// Creates a minimal snapshot row if none exists yet.
func (s *Store) SetSnapshotStatus(ctx context.Context, contextID, slug string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_snapshots (context_id, status)
		VALUES (?, ?)
		ON CONFLICT(context_id) DO UPDATE SET status = excluded.status
	`, contextID, slug)
	if err != nil {
		return fmt.Errorf("set snapshot status: %w", err)
	}
	return nil
}
