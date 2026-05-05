package vcsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RepoPathRepo handles persistence for vcs_repo_paths rows.
type RepoPathRepo struct {
	db *sql.DB
}

// NewRepoPathRepo creates a RepoPathRepo.
func NewRepoPathRepo(db *sql.DB) *RepoPathRepo { return &RepoPathRepo{db: db} }

// Add inserts a new path for the given repo. Returns an error if the (repo_id, path) pair already exists.
func (r *RepoPathRepo) Add(ctx context.Context, repoID, path string) (*VCSRepoPath, error) {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO vcs_repo_paths (id, repo_id, path, created_at) VALUES (?, ?, ?, ?)`,
		id, repoID, path, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert vcs_repo_path: %w", err)
	}
	return &VCSRepoPath{ID: id, RepoID: repoID, Path: path, CreatedAt: now}, nil
}

// ListForRepo returns all paths registered for a given repo, ordered by created_at.
func (r *RepoPathRepo) ListForRepo(ctx context.Context, repoID string) ([]VCSRepoPath, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, repo_id, path, created_at FROM vcs_repo_paths WHERE repo_id = ? ORDER BY created_at`,
		repoID,
	)
	if err != nil {
		return nil, fmt.Errorf("list vcs_repo_paths: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []VCSRepoPath
	for rows.Next() {
		var p VCSRepoPath
		if err := rows.Scan(&p.ID, &p.RepoID, &p.Path, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vcs_repo_path: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// Delete removes a path by ID. Returns ErrNotFound if no row was deleted.
func (r *RepoPathRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM vcs_repo_paths WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete vcs_repo_path: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// getByID returns a path row by its primary key. Used internally.
func (r *RepoPathRepo) getByID(ctx context.Context, id string) (*VCSRepoPath, error) {
	var p VCSRepoPath
	err := r.db.QueryRowContext(ctx,
		`SELECT id, repo_id, path, created_at FROM vcs_repo_paths WHERE id = ?`, id,
	).Scan(&p.ID, &p.RepoID, &p.Path, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vcs_repo_path: %w", err)
	}
	return &p, nil
}
