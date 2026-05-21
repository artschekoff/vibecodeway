package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateUser inserts a new org-level user and returns it.
func (s *Store) CreateUser(ctx context.Context, args CreateUserArgs) (*User, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	u := &User{
		ID:          uuid.New().String(),
		DisplayName: args.DisplayName,
		Email:       args.Email,
		AvatarURL:   args.AvatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	var orgPtr *string
	if args.OrgID != "" {
		orgPtr = &args.OrgID
		u.OrgID = orgPtr
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, org_id, display_name, email, avatar_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, u.ID, orgPtr, u.DisplayName, u.Email, u.AvatarURL, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// GetUser returns a user by ID. Returns ErrNotFound if not found.
func (s *Store) GetUser(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, org_id, display_name, email, avatar_url, created_at, updated_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.OrgID, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

// ListUsers returns all org users, optionally filtered by orgID (empty = all orgs).
func (s *Store) ListUsers(ctx context.Context, orgID string) ([]User, error) {
	q := `SELECT id, org_id, display_name, email, avatar_url, created_at, updated_at FROM users`
	var args []any
	if orgID != "" {
		q += " WHERE org_id = ?"
		args = append(args, orgID)
	}
	q += " ORDER BY display_name ASC"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
