package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// TourStep is a single stop in a code tour: a code location plus the
// what/why explanation for the change there. Anchors are snippets, not line
// numbers — the UI locates the line by searching the current file content,
// so user edits mid-tour do not break positioning.
type TourStep struct {
	File   string `json:"file"`   // repo-relative path
	Anchor string `json:"anchor"` // unique code snippet to position on
	What   string `json:"what"`
	Why    string `json:"why"`
	// Review is the reviewer remark this step addresses (PR review tours):
	// "@author path:line: body". Empty for ordinary tours.
	Review string `json:"review,omitempty"`
}

// CodeTour is a guided tour through an agent's changes for one (context,
// repo) pair. Only one tour is active per pair: starting a new tour replaces
// the previous row. Persistence lets the tour survive tab switches and app
// restarts — position and per-step decisions included.
type CodeTour struct {
	ID        string         `json:"id"`
	ContextID string         `json:"contextId"`
	RepoPath  string         `json:"repoPath"`
	Source    string         `json:"source"` // "agent" | "generated"
	Title     string         `json:"title"`
	Steps     []TourStep     `json:"steps"`
	StepIndex int            `json:"stepIndex"`
	Decisions map[int]string `json:"decisions"` // stepIdx -> "staged"|"discarded"|"edited"|"skipped"
	CreatedAt string         `json:"createdAt"`
}

// UpsertCodeTour replaces any existing tour for (ContextID, RepoPath) with
// tour, inside a transaction so the delete-then-insert is atomic. This
// enforces "one active tour per (context, repo)" from the spec.
func (s *Store) UpsertCodeTour(ctx context.Context, tour *CodeTour) error {
	if tour.ID == "" {
		tour.ID = uuid.New().String()
	}
	if tour.Decisions == nil {
		tour.Decisions = map[int]string{}
	}

	stepsRaw, err := json.Marshal(tour.Steps)
	if err != nil {
		return fmt.Errorf("marshal tour steps: %w", err)
	}
	decisionsRaw, err := json.Marshal(tour.Decisions)
	if err != nil {
		return fmt.Errorf("marshal tour decisions: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert code tour tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM code_tours WHERE context_id = ? AND repo_path = ?`,
		tour.ContextID, tour.RepoPath,
	); err != nil {
		return fmt.Errorf("delete existing code tour: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO code_tours (id, context_id, repo_path, source, title, steps_json, step_index, decisions_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tour.ID, tour.ContextID, tour.RepoPath, tour.Source, tour.Title,
		string(stepsRaw), tour.StepIndex, string(decisionsRaw), tour.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert code tour: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert code tour: %w", err)
	}
	return nil
}

// GetCodeTour returns the active tour for (contextID, repoPath), or nil, nil
// if none exists.
func (s *Store) GetCodeTour(ctx context.Context, contextID, repoPath string) (*CodeTour, error) {
	var (
		tour         CodeTour
		stepsRaw     string
		decisionsRaw string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, context_id, repo_path, source, title, steps_json, step_index, decisions_json, created_at
		FROM code_tours
		WHERE context_id = ? AND repo_path = ?
	`, contextID, repoPath).Scan(
		&tour.ID, &tour.ContextID, &tour.RepoPath, &tour.Source, &tour.Title,
		&stepsRaw, &tour.StepIndex, &decisionsRaw, &tour.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get code tour: %w", err)
	}

	if err := json.Unmarshal([]byte(stepsRaw), &tour.Steps); err != nil {
		return nil, fmt.Errorf("unmarshal tour steps: %w", err)
	}
	tour.Decisions = map[int]string{}
	if err := json.Unmarshal([]byte(decisionsRaw), &tour.Decisions); err != nil {
		return nil, fmt.Errorf("unmarshal tour decisions: %w", err)
	}
	return &tour, nil
}

// SetCodeTourStep updates the current step index for tour id.
func (s *Store) SetCodeTourStep(ctx context.Context, id string, stepIndex int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE code_tours SET step_index = ? WHERE id = ?`,
		stepIndex, id,
	)
	if err != nil {
		return fmt.Errorf("set code tour step: %w", err)
	}
	return nil
}

// SetCodeTourDecision records the decision (staged/discarded/edited/skipped)
// for step stepIdx of tour id, read-modify-write on decisions_json.
func (s *Store) SetCodeTourDecision(ctx context.Context, id string, stepIdx int, decision string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set code tour decision tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var decisionsRaw string
	if err := tx.QueryRowContext(ctx,
		`SELECT decisions_json FROM code_tours WHERE id = ?`, id,
	).Scan(&decisionsRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("get code tour decisions: %w", err)
	}

	decisions := map[int]string{}
	if err := json.Unmarshal([]byte(decisionsRaw), &decisions); err != nil {
		return fmt.Errorf("unmarshal tour decisions: %w", err)
	}
	decisions[stepIdx] = decision

	updatedRaw, err := json.Marshal(decisions)
	if err != nil {
		return fmt.Errorf("marshal tour decisions: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE code_tours SET decisions_json = ? WHERE id = ?`,
		string(updatedRaw), id,
	); err != nil {
		return fmt.Errorf("update code tour decisions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit set code tour decision: %w", err)
	}
	return nil
}
