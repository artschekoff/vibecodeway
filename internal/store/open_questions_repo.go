package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *Store) CreateOpenQuestion(ctx context.Context, contextID, question, source string) (*OpenQuestion, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	q := &OpenQuestion{
		ID:        uuid.New().String(),
		ContextID: contextID,
		Question:  question,
		Status:    "open",
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_open_questions (id, context_id, question, status, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, q.ID, q.ContextID, q.Question, q.Status, q.Source, q.CreatedAt, q.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create open question: %w", err)
	}
	return q, nil
}

func (s *Store) ListOpenQuestions(ctx context.Context, contextID string) ([]OpenQuestion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, question, status, answer, source, created_at, updated_at, closed_at
		FROM context_open_questions WHERE context_id = ? ORDER BY created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list open questions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanOpenQuestions(rows)
}

func (s *Store) ListOpenOpenQuestions(ctx context.Context, contextID string) ([]OpenQuestion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, question, status, answer, source, created_at, updated_at, closed_at
		FROM context_open_questions WHERE context_id = ? AND status = 'open' ORDER BY created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list open open questions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanOpenQuestions(rows)
}

func (s *Store) AnswerOpenQuestion(ctx context.Context, id, answer string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE context_open_questions SET status='answered', answer=?, updated_at=?, closed_at=? WHERE id=?
	`, answer, now, now, id)
	if err != nil {
		return fmt.Errorf("answer open question: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DismissOpenQuestion(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
		UPDATE context_open_questions SET status='dismissed', updated_at=?, closed_at=? WHERE id=?
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("dismiss open question: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) HasOpenQuestions(ctx context.Context, contextID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM context_open_questions WHERE context_id=? AND status='open'`, contextID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("has open questions: %w", err)
	}
	return count > 0, nil
}

// IsFirstRefresh returns true when no context_logs exist for this context yet.
func (s *Store) IsFirstRefresh(ctx context.Context, contextID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM context_logs WHERE context_id=?`, contextID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("is first refresh: %w", err)
	}
	return count == 0, nil
}

func scanOpenQuestions(rows *sql.Rows) ([]OpenQuestion, error) {
	var result []OpenQuestion
	for rows.Next() {
		var q OpenQuestion
		if err := rows.Scan(&q.ID, &q.ContextID, &q.Question, &q.Status, &q.Answer,
			&q.Source, &q.CreatedAt, &q.UpdatedAt, &q.ClosedAt); err != nil {
			return nil, fmt.Errorf("scan open question: %w", err)
		}
		result = append(result, q)
	}
	return result, rows.Err()
}
