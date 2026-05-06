package vcsstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WorkspaceRepo handles persistence for vcs_workspaces rows.
type WorkspaceRepo struct {
	db *sql.DB
}

// NewWorkspaceRepo creates a WorkspaceRepo.
func NewWorkspaceRepo(db *sql.DB) *WorkspaceRepo { return &WorkspaceRepo{db: db} }

// Create inserts a new workspace record.
func (r *WorkspaceRepo) Create(ctx context.Context, w *VCSWorkspace) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	w.CreatedAt = now
	w.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO vcs_workspaces (id, repo_path_id, repo_id, branch, worktree_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.RepoPathID, w.RepoID, w.Branch, w.WorktreePath, w.Status, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert vcs_workspace: %w", err)
	}
	return nil
}

// Get returns a workspace by ID.
func (r *WorkspaceRepo) Get(ctx context.Context, id string) (*VCSWorkspace, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, repo_path_id, repo_id, branch, worktree_path, status, created_at, updated_at
		 FROM vcs_workspaces WHERE id = ?`, id)
	w, err := scanWorkspace(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return w, err
}

// FindByBranch returns the first workspace for the given repo and branch, or nil if none.
func (r *WorkspaceRepo) FindByBranch(ctx context.Context, repoID, branch string) (*VCSWorkspace, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, repo_path_id, repo_id, branch, worktree_path, status, created_at, updated_at
		 FROM vcs_workspaces WHERE repo_id = ? AND branch = ? LIMIT 1`,
		repoID, branch)
	w, err := scanWorkspace(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return w, err
}

// ListByPath returns all workspaces for a specific repo path, newest first.
func (r *WorkspaceRepo) ListByPath(ctx context.Context, repoPathID string) ([]VCSWorkspace, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, repo_path_id, repo_id, branch, worktree_path, status, created_at, updated_at
		 FROM vcs_workspaces WHERE repo_path_id = ? ORDER BY created_at DESC`, repoPathID)
	if err != nil {
		return nil, fmt.Errorf("list vcs_workspaces by path: %w", err)
	}
	return scanWorkspaces(rows)
}

// ListByRepo returns all workspaces for a repo (across all its paths), newest first.
func (r *WorkspaceRepo) ListByRepo(ctx context.Context, repoID string) ([]VCSWorkspace, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, repo_path_id, repo_id, branch, worktree_path, status, created_at, updated_at
		 FROM vcs_workspaces WHERE repo_id = ? ORDER BY created_at DESC`, repoID)
	if err != nil {
		return nil, fmt.Errorf("list vcs_workspaces by repo: %w", err)
	}
	return scanWorkspaces(rows)
}

// ListAll returns every workspace, newest first.
func (r *WorkspaceRepo) ListAll(ctx context.Context) ([]VCSWorkspace, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, repo_path_id, repo_id, branch, worktree_path, status, created_at, updated_at
		 FROM vcs_workspaces ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all vcs_workspaces: %w", err)
	}
	return scanWorkspaces(rows)
}

// UpdateStatus sets the status field for a workspace.
func (r *WorkspaceRepo) UpdateStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		`UPDATE vcs_workspaces SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id)
	if err != nil {
		return fmt.Errorf("update vcs_workspace status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a workspace by ID.
func (r *WorkspaceRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM vcs_workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete vcs_workspace: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByWorktreePath removes a workspace by its filesystem path.
func (r *WorkspaceRepo) DeleteByWorktreePath(ctx context.Context, path string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM vcs_workspaces WHERE worktree_path = ?`, path)
	return err
}

func scanWorkspace(row *sql.Row) (*VCSWorkspace, error) {
	var w VCSWorkspace
	err := row.Scan(&w.ID, &w.RepoPathID, &w.RepoID, &w.Branch, &w.WorktreePath, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func scanWorkspaces(rows *sql.Rows) ([]VCSWorkspace, error) {
	defer func() { _ = rows.Close() }()
	var result []VCSWorkspace
	for rows.Next() {
		var w VCSWorkspace
		if err := rows.Scan(&w.ID, &w.RepoPathID, &w.RepoID, &w.Branch, &w.WorktreePath, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan vcs_workspace: %w", err)
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
