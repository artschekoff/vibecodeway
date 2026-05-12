package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AppendEstimationLog inserts one estimation change record.
// ID and RecordedAt are set automatically if empty.
func (s *Store) AppendEstimationLog(ctx context.Context, e EstimationLog) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.RecordedAt == "" {
		e.RecordedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO estimation_log
		  (id, context_id, recorded_at, trigger_type, trigger_description,
		   delta_hours, estimated_hours_before, estimated_hours_after,
		   estimated_completion_before, estimated_completion_after, estimation_basis)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		e.ID, e.ContextID, e.RecordedAt, e.TriggerType, e.TriggerDescription,
		e.DeltaHours, e.EstimatedHoursBefore, e.EstimatedHoursAfter,
		e.EstimatedCompletionBefore, e.EstimatedCompletionAfter, e.EstimationBasis,
	)
	if err != nil {
		return fmt.Errorf("append estimation log: %w", err)
	}
	return nil
}

// GetEstimationHistory returns all log entries for a context, newest first.
func (s *Store) GetEstimationHistory(ctx context.Context, contextID string) ([]EstimationLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, recorded_at, trigger_type, trigger_description,
		       delta_hours, estimated_hours_before, estimated_hours_after,
		       estimated_completion_before, estimated_completion_after, estimation_basis
		FROM estimation_log
		WHERE context_id = ?
		ORDER BY recorded_at DESC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("get estimation history: %w", err)
	}
	defer rows.Close()

	var result []EstimationLog
	for rows.Next() {
		var e EstimationLog
		var (
			deltaHours                sql.NullFloat64
			estimatedHoursBefore      sql.NullFloat64
			estimatedHoursAfter       sql.NullFloat64
			estimatedCompletionBefore sql.NullString
			estimatedCompletionAfter  sql.NullString
		)
		if err := rows.Scan(
			&e.ID, &e.ContextID, &e.RecordedAt, &e.TriggerType, &e.TriggerDescription,
			&deltaHours, &estimatedHoursBefore, &estimatedHoursAfter,
			&estimatedCompletionBefore, &estimatedCompletionAfter, &e.EstimationBasis,
		); err != nil {
			return nil, fmt.Errorf("scan estimation log: %w", err)
		}
		if deltaHours.Valid {
			v := deltaHours.Float64
			e.DeltaHours = &v
		}
		if estimatedHoursBefore.Valid {
			v := estimatedHoursBefore.Float64
			e.EstimatedHoursBefore = &v
		}
		if estimatedHoursAfter.Valid {
			v := estimatedHoursAfter.Float64
			e.EstimatedHoursAfter = &v
		}
		if estimatedCompletionBefore.Valid {
			e.EstimatedCompletionBefore = &estimatedCompletionBefore.String
		}
		if estimatedCompletionAfter.Valid {
			e.EstimatedCompletionAfter = &estimatedCompletionAfter.String
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
