package vcsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RepoRepo handles persistence for vcs_repos rows.
type RepoRepo struct {
	db *sql.DB
}

// NewRepoRepo creates a RepoRepo.
func NewRepoRepo(db *sql.DB) *RepoRepo { return &RepoRepo{db: db} }

// GetRepo returns a repo by ID.
func (r *RepoRepo) GetRepo(ctx context.Context, id string) (*VCSRepo, error) {
	var repo VCSRepo
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, platform, owner, name, local_path, label, description, on_checkout_command, created_at FROM vcs_repos WHERE id = ?`, id,
	).Scan(&repo.ID, &repo.Platform, &repo.Owner, &repo.Name, &repo.LocalPath, &repo.Label, &repo.Description, &repo.OnCheckoutCommand, &repo.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vcs repo: %w", err)
	}
	return &repo, nil
}

// EnsureRepo returns the existing repo for (platform, owner, name) or creates it.
func (r *RepoRepo) EnsureRepo(ctx context.Context, platform, owner, name string) (*VCSRepo, error) {
	var repo VCSRepo
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, platform, owner, name, local_path, label, description, on_checkout_command, created_at FROM vcs_repos WHERE platform = ? AND lower(owner) = lower(?) AND lower(name) = lower(?)`,
		platform, owner, name,
	).Scan(&repo.ID, &repo.Platform, &repo.Owner, &repo.Name, &repo.LocalPath, &repo.Label, &repo.Description, &repo.OnCheckoutCommand, &repo.CreatedAt)
	if err == nil {
		return &repo, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query vcs repo: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO vcs_repos (id, platform, owner, name, local_path, label, description, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, platform, owner, name, "", "", "", now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert vcs repo: %w", err)
	}
	return &VCSRepo{ID: id, Platform: platform, Owner: owner, Name: name, LocalPath: "", Label: "", Description: "", CreatedAt: now}, nil
}

// ListRepos returns all tracked repos, optionally filtered by platform.
func (r *RepoRepo) ListRepos(ctx context.Context, platform string) ([]VCSRepo, error) {
	q := `SELECT id, platform, owner, name, local_path, label, description, on_checkout_command, created_at FROM vcs_repos`
	var args []any
	if platform != "" {
		q += " WHERE platform = ?"
		args = append(args, platform)
	}
	q += " ORDER BY platform, owner, name"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list vcs repos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []VCSRepo
	for rows.Next() {
		var repo VCSRepo
		if err := rows.Scan(&repo.ID, &repo.Platform, &repo.Owner, &repo.Name, &repo.LocalPath, &repo.Label, &repo.Description, &repo.OnCheckoutCommand, &repo.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan vcs repo: %w", err)
		}
		result = append(result, repo)
	}
	return result, rows.Err()
}

// UpdateLocalPath sets the local filesystem path for a tracked repo.
func (r *RepoRepo) UpdateLocalPath(ctx context.Context, id, localPath string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE vcs_repos SET local_path = ? WHERE id = ?`, localPath, id)
	if err != nil {
		return fmt.Errorf("update vcs repo local_path: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRepo removes a repo by ID. Branches and PRs cascade-delete.
func (r *RepoRepo) DeleteRepo(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM vcs_repos WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete vcs repo: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateRepoMeta updates the label, description, and on-checkout command for a tracked repo.
func (r *RepoRepo) UpdateRepoMeta(ctx context.Context, id, label, description, onCheckoutCommand string) error {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE vcs_repos SET label = ?, description = ?, on_checkout_command = ? WHERE id = ?`, label, description, onCheckoutCommand, id,
	)
	if err != nil {
		return fmt.Errorf("update vcs repo meta: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
