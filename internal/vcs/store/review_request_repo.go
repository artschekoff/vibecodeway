package vcsstore

import (
	"context"
	"database/sql"
	"encoding/json"
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
// When an existing record is updated (PR re-appears in the review list), reviewed_at and
// dismissed_at are cleared so it shows as a fresh pending review again.
func (r *ReviewRequestRepo) UpsertReviewRequest(ctx context.Context, repoID string, prNumber int, prTitle, prAuthor, prURL, reviewStatus, headBranch, requestedAt, reviewersJSON string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if reviewersJSON == "" {
		reviewersJSON = "[]"
	}

	var existing string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM vcs_review_requests WHERE repo_id = ? AND pr_number = ?`,
		repoID, prNumber,
	).Scan(&existing)

	if err == nil {
		// Existing row: refresh fields + reviewers, clear reviewed/dismissed, KEEP viewed_at.
		_, err = r.db.ExecContext(ctx, `
			UPDATE vcs_review_requests
			SET pr_title = ?, pr_author = ?, pr_url = ?, review_status = ?,
			    head_branch = ?, requested_at = ?, fetched_at = ?, reviewers = ?,
			    reviewed_at = NULL, dismissed_at = NULL
			WHERE id = ?
		`, prTitle, prAuthor, prURL, reviewStatus, headBranch, requestedAt, now, reviewersJSON, existing)
		return false, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("query review request: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO vcs_review_requests
			(id, repo_id, pr_number, pr_title, pr_author, pr_url,
			 review_status, head_branch, requested_at, fetched_at, reviewers)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, repoID, prNumber, prTitle, prAuthor, prURL, reviewStatus, headBranch, requestedAt, now, reviewersJSON)
	if err != nil {
		return false, fmt.Errorf("insert review request: %w", err)
	}
	return true, nil
}

// MarkReviewedForRepo marks all review requests for a repo that are NOT in the given
// set of active PR numbers as "reviewed" (i.e. the user submitted a review and they
// disappeared from GitHub's review-requested list). Records that are already dismissed
// or already have reviewed_at are left unchanged.
func (r *ReviewRequestRepo) MarkReviewedForRepo(ctx context.Context, repoID string, activePRNumbers []int) error {
	now := time.Now().UTC().Format(time.RFC3339)

	// Build a NOT IN clause.
	if len(activePRNumbers) == 0 {
		// No active PRs — mark everything as reviewed.
		_, err := r.db.ExecContext(ctx, `
			UPDATE vcs_review_requests
			SET reviewed_at = ?
			WHERE repo_id = ? AND reviewed_at IS NULL AND dismissed_at IS NULL
		`, now, repoID)
		return err
	}

	// Build placeholder list.
	placeholders := make([]any, 0, len(activePRNumbers)+2)
	placeholders = append(placeholders, now, repoID)
	inClause := "("
	for i, n := range activePRNumbers {
		if i > 0 {
			inClause += ","
		}
		inClause += "?"
		placeholders = append(placeholders, n)
	}
	inClause += ")"

	_, err := r.db.ExecContext(ctx, `
		UPDATE vcs_review_requests
		SET reviewed_at = ?
		WHERE repo_id = ? AND pr_number NOT IN `+inClause+`
		  AND reviewed_at IS NULL AND dismissed_at IS NULL
	`, placeholders...)
	return err
}

// DismissReviewRequest marks a review request as dismissed (hides it from "waiting for author").
func (r *ReviewRequestRepo) DismissReviewRequest(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx,
		`UPDATE vcs_review_requests SET dismissed_at = ? WHERE id = ?`, now, id)
	return err
}

// MarkReviewRequestViewed records that the user has looked at a review item.
func (r *ReviewRequestRepo) MarkReviewRequestViewed(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx,
		`UPDATE vcs_review_requests SET viewed_at = ? WHERE id = ? AND viewed_at IS NULL`, now, id)
	return err
}

// MarkAllReviewRequestsViewed marks every non-dismissed review item as viewed.
func (r *ReviewRequestRepo) MarkAllReviewRequestsViewed(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx,
		`UPDATE vcs_review_requests SET viewed_at = ? WHERE viewed_at IS NULL AND dismissed_at IS NULL`, now)
	return err
}

// SetReviewStatus updates only the review_status for a stored review request,
// leaving reviewed_at/dismissed_at untouched. Used to relabel dropped PRs as
// "merged"/"closed" without changing their waiting state.
func (r *ReviewRequestRepo) SetReviewStatus(ctx context.Context, repoID string, prNumber int, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE vcs_review_requests SET review_status = ? WHERE repo_id = ? AND pr_number = ?`,
		status, repoID, prNumber)
	return err
}

// ListReviewRequests returns non-dismissed review requests, optionally filtered by repoID.
func (r *ReviewRequestRepo) ListReviewRequests(ctx context.Context, repoID string) ([]ReviewRequest, error) {
	q := `
		SELECT id, repo_id, pr_number, pr_title, pr_author, pr_url,
		       review_status, head_branch, requested_at, fetched_at, reviewed_at, dismissed_at,
		       viewed_at, reviewers
		FROM vcs_review_requests
		WHERE dismissed_at IS NULL`
	var args []any
	if repoID != "" {
		q += " AND repo_id = ?"
		args = append(args, repoID)
	}
	q += " ORDER BY reviewed_at ASC, fetched_at DESC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list review requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []ReviewRequest
	for rows.Next() {
		var rr ReviewRequest
		var reviewersJSON string
		if err := rows.Scan(
			&rr.ID, &rr.RepoID, &rr.PRNumber, &rr.PRTitle, &rr.PRAuthor,
			&rr.PRURL, &rr.ReviewStatus, &rr.HeadBranch, &rr.RequestedAt, &rr.FetchedAt,
			&rr.ReviewedAt, &rr.DismissedAt, &rr.ViewedAt, &reviewersJSON,
		); err != nil {
			return nil, fmt.Errorf("scan review request: %w", err)
		}
		if reviewersJSON != "" && reviewersJSON != "[]" {
			_ = json.Unmarshal([]byte(reviewersJSON), &rr.Reviewers)
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
		&rr.PRURL, &rr.ReviewStatus, &rr.HeadBranch, &rr.RequestedAt, &rr.FetchedAt,
		&rr.ReviewedAt, &rr.DismissedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rr, nil
}
