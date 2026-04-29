package vcsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BranchRepo handles persistence for context_vcs_branches rows.
type BranchRepo struct {
	db *sql.DB
}

// NewBranchRepo creates a BranchRepo.
func NewBranchRepo(db *sql.DB) *BranchRepo { return &BranchRepo{db: db} }

// AddBranch inserts a new branch link. Returns ErrNotFound if the repo doesn't exist.
// Returns the existing row if the (contextID, repoID, branch) tuple already exists.
func (r *BranchRepo) AddBranch(ctx context.Context, contextID, repoID, branch, label string) (*ContextVCSBranch, error) {
	// Check for existing
	var b ContextVCSBranch
	err := r.db.QueryRowContext(ctx,
		`SELECT id, context_id, repo_id, branch, label, created_at FROM context_vcs_branches
		 WHERE context_id = ? AND repo_id = ? AND branch = ?`,
		contextID, repoID, branch,
	).Scan(&b.ID, &b.ContextID, &b.RepoID, &b.Branch, &b.Label, &b.CreatedAt)
	if err == nil {
		// Update label if provided
		if label != "" && label != b.Label {
			_, _ = r.db.ExecContext(ctx, `UPDATE context_vcs_branches SET label = ? WHERE id = ?`, label, b.ID)
			b.Label = label
		}
		return &b, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query vcs branch: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO context_vcs_branches (id, context_id, repo_id, branch, label, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, contextID, repoID, branch, label, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert vcs branch: %w", err)
	}
	return &ContextVCSBranch{ID: id, ContextID: contextID, RepoID: repoID, Branch: branch, Label: label, CreatedAt: now}, nil
}

// GetBranch returns a branch by ID.
func (r *BranchRepo) GetBranch(ctx context.Context, id string) (*ContextVCSBranch, error) {
	var b ContextVCSBranch
	err := r.db.QueryRowContext(ctx,
		`SELECT id, context_id, repo_id, branch, label, created_at FROM context_vcs_branches WHERE id = ?`, id,
	).Scan(&b.ID, &b.ContextID, &b.RepoID, &b.Branch, &b.Label, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vcs branch: %w", err)
	}
	return &b, nil
}

// ListBranches returns all branches for a context, with repo info joined.
func (r *BranchRepo) ListBranches(ctx context.Context, contextID string) ([]ContextVCSBranch, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.id, b.context_id, b.repo_id, b.branch, b.label, b.created_at,
		       rp.platform, rp.owner, rp.name, rp.created_at
		FROM context_vcs_branches b
		JOIN vcs_repos rp ON rp.id = b.repo_id
		WHERE b.context_id = ?
		ORDER BY b.created_at ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list vcs branches: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []ContextVCSBranch
	for rows.Next() {
		var b ContextVCSBranch
		var repo VCSRepo
		if err := rows.Scan(
			&b.ID, &b.ContextID, &b.RepoID, &b.Branch, &b.Label, &b.CreatedAt,
			&repo.Platform, &repo.Owner, &repo.Name, &repo.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan vcs branch: %w", err)
		}
		repo.ID = b.RepoID
		b.Repo = &repo
		result = append(result, b)
	}
	return result, rows.Err()
}

// ListAllBranchesForRepo returns all context branches for a repo.
func (r *BranchRepo) ListAllBranchesForRepo(ctx context.Context, repoID string) ([]ContextVCSBranch, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, context_id, repo_id, branch, label, created_at FROM context_vcs_branches
		 WHERE repo_id = ? ORDER BY created_at ASC`,
		repoID,
	)
	if err != nil {
		return nil, fmt.Errorf("list branches for repo: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []ContextVCSBranch
	for rows.Next() {
		var b ContextVCSBranch
		if err := rows.Scan(&b.ID, &b.ContextID, &b.RepoID, &b.Branch, &b.Label, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vcs branch: %w", err)
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// DeleteBranch removes a branch by ID. PRs cascade-delete.
func (r *BranchRepo) DeleteBranch(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM context_vcs_branches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete vcs branch: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateBranchLabel sets the human-readable label for a branch link (e.g. "api", "data").
func (r *BranchRepo) UpdateBranchLabel(ctx context.Context, id, label string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE context_vcs_branches SET label = ? WHERE id = ?`, label, id)
	if err != nil {
		return fmt.Errorf("update branch label: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
