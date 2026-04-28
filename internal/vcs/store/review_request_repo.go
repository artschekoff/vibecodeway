package vcsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReviewRequestRepo handles persistence for vcs_review_requests rows.
type ReviewRequestRepo struct {
	db *sql.DB
}

// NewReviewRequestRepo creates a ReviewRequestRepo.
func NewReviewRequestRepo(db *sql.DB) *ReviewRequestRepo { return &ReviewRequestRepo{db: db} }

// UpsertReviewRequest creates or updates a review request identified by (repoID, prNumber).
func (r *ReviewRequestRepo) UpsertReviewRequest(ctx context.Context, repoID string, prNumber int, prTitle, prAuthor, prURL, reviewStatus, requestedAt string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM vcs_review_requests WHERE repo_id = ? AND pr_number = ?`,
		repoID, prNumber,
	).Scan(&existing)

	if err == nil {
		_, err = r.db.ExecContext(ctx, `
			UPDATE vcs_review_requests
			SET pr_title = ?, pr_author = ?, pr_url = ?, review_status = ?, requested_at = ?, fetched_at = ?
			WHERE id = ?
		`, prTitle, prAuthor, prURL, reviewStatus, requestedAt, now, existing)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query review request: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO vcs_review_requests (id, repo_id, pr_number, pr_title, pr_author, pr_url, review_status, requested_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, repoID, prNumber, prTitle, prAuthor, prURL, reviewStatus, requestedAt, now)
	if err != nil {
		return fmt.Errorf("insert review request: %w", err)
	}
	return nil
}

// ListReviewRequests returns all pending review requests, optionally filtered by repoID.
func (r *ReviewRequestRepo) ListReviewRequests(ctx context.Context, repoID string) ([]ReviewRequest, error) {
	q := `
		SELECT id, repo_id, pr_number, pr_title, pr_author, pr_url, review_status, requested_at, fetched_at
		FROM vcs_review_requests`
	var args []any
	if repoID != "" {
		q += " WHERE repo_id = ?"
		args = append(args, repoID)
	}
	q += " ORDER BY fetched_at DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list review requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []ReviewRequest
	for rows.Next() {
		var rr ReviewRequest
		if err := rows.Scan(
			&rr.ID, &rr.RepoID, &rr.PRNumber, &rr.PRTitle, &rr.PRAuthor,
			&rr.PRURL, &rr.ReviewStatus, &rr.RequestedAt, &rr.FetchedAt,
		); err != nil {
			return nil, fmt.Errorf("scan review request: %w", err)
		}
		result = append(result, rr)
	}
	return result, rows.Err()
}

// DeleteReviewRequestsForRepo removes all review requests for a repo (for full refresh).
func (r *ReviewRequestRepo) DeleteReviewRequestsForRepo(ctx context.Context, repoID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM vcs_review_requests WHERE repo_id = ?`, repoID)
	if err != nil {
		return fmt.Errorf("delete review requests: %w", err)
	}
	return nil
}

// DeletePRsForBranch removes all PR rows for a specific branch.
// Used when a branch is removed.
func (r *ReviewRequestRepo) DeletePRsForBranch(ctx context.Context, branchID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM vcs_pull_requests WHERE branch_id = ?`, branchID)
	if err != nil {
		return fmt.Errorf("delete prs for branch: %w", err)
	}
	return nil
}

// scanReviewRequest scans a single review request row.
func scanReviewRequest(row *sql.Row) (*ReviewRequest, error) {
	var rr ReviewRequest
	err := row.Scan(
		&rr.ID, &rr.RepoID, &rr.PRNumber, &rr.PRTitle, &rr.PRAuthor,
		&rr.PRURL, &rr.ReviewStatus, &rr.RequestedAt, &rr.FetchedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rr, nil
}
