package vcsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AuthRepo handles persistence for vcs_auth rows.
type AuthRepo struct {
	db *sql.DB
}

// NewAuthRepo creates an AuthRepo.
func NewAuthRepo(db *sql.DB) *AuthRepo { return &AuthRepo{db: db} }

// GetAuth returns the auth row for the given platform.
// Returns ErrNotFound if no row exists.
func (r *AuthRepo) GetAuth(ctx context.Context, platform string) (*VCSAuth, error) {
	var a VCSAuth
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, platform, username, avatar_url, token, updated_at FROM vcs_auth WHERE platform = ?`,
		platform,
	).Scan(&a.ID, &a.Platform, &a.Username, &a.AvatarURL, &a.Token, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vcs auth: %w", err)
	}
	return &a, nil
}

// UpsertAuth creates or updates the auth row for a platform.
// token may be empty to preserve the existing stored token (on update) or store no token (on insert).
func (r *AuthRepo) UpsertAuth(ctx context.Context, platform, username, avatarURL, token string) (*VCSAuth, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing VCSAuth
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id FROM vcs_auth WHERE platform = ?`, platform,
	).Scan(&existing.ID)

	if err == nil {
		if token != "" {
			_, err = r.db.ExecContext(
				ctx,
				`UPDATE vcs_auth SET username = ?, avatar_url = ?, token = ?, updated_at = ? WHERE platform = ?`,
				username, avatarURL, token, now, platform,
			)
		} else {
			_, err = r.db.ExecContext(
				ctx,
				`UPDATE vcs_auth SET username = ?, avatar_url = ?, updated_at = ? WHERE platform = ?`,
				username, avatarURL, now, platform,
			)
		}
		if err != nil {
			return nil, fmt.Errorf("update vcs auth: %w", err)
		}
		return r.GetAuth(ctx, platform)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query vcs auth: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO vcs_auth (id, platform, username, avatar_url, token, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, platform, username, avatarURL, token, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert vcs auth: %w", err)
	}
	return r.GetAuth(ctx, platform)
}

// DeleteAuth removes the auth row for a platform.
func (r *AuthRepo) DeleteAuth(ctx context.Context, platform string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM vcs_auth WHERE platform = ?`, platform)
	if err != nil {
		return fmt.Errorf("delete vcs auth: %w", err)
	}
	return nil
}

// ListAuth returns all stored auth rows.
func (r *AuthRepo) ListAuth(ctx context.Context) ([]VCSAuth, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, platform, username, avatar_url, token, updated_at FROM vcs_auth ORDER BY platform`,
	)
	if err != nil {
		return nil, fmt.Errorf("list vcs auth: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []VCSAuth
	for rows.Next() {
		var a VCSAuth
		if err := rows.Scan(&a.ID, &a.Platform, &a.Username, &a.AvatarURL, &a.Token, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan vcs auth: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
