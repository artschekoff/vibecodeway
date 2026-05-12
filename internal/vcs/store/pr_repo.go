package vcsstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PRRepo handles persistence for vcs_pull_requests and vcs_pr_comments.
type PRRepo struct {
	db *sql.DB
}

// NewPRRepo creates a PRRepo.
func NewPRRepo(db *sql.DB) *PRRepo { return &PRRepo{db: db} }

// PRUpsertResult holds the outcome of an UpsertPR call, including detected state changes.
type PRUpsertResult struct {
	ID                  string
	IsNew               bool
	StateChanged        bool
	OldState            string
	MergeableChanged    bool
	OldMergeable        string
	ReviewStatusChanged bool
	OldReviewStatus     string
	CIStatusChanged     bool
	OldCIStatus         string
}

// UpsertPR creates or updates a pull request row identified by (repoID, prNumber).
// Returns a PRUpsertResult describing what changed.
func (r *PRRepo) UpsertPR(ctx context.Context, repoID, branchID string, prNumber int, title, author, state, reviewStatus, url, headBranch, mergeable, ciStatus, prCreatedAt, prUpdatedAt string) (PRUpsertResult, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existing, oldState, oldMergeable, oldReviewStatus, oldCIStatus string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, state, mergeable, review_status, ci_status FROM vcs_pull_requests WHERE repo_id = ? AND pr_number = ?`,
		repoID, prNumber,
	).Scan(&existing, &oldState, &oldMergeable, &oldReviewStatus, &oldCIStatus)

	if err == nil {
		_, err = r.db.ExecContext(ctx, `
			UPDATE vcs_pull_requests
			SET branch_id = ?, title = ?, author = ?, state = ?, review_status = ?,
			    url = ?, head_branch = ?, mergeable = ?, ci_status = ?,
			    pr_created_at = ?, pr_updated_at = ?, fetched_at = ?
			WHERE id = ?
		`, branchID, title, author, state, reviewStatus, url, headBranch, mergeable, ciStatus,
			prCreatedAt, prUpdatedAt, now, existing)
		if err != nil {
			return PRUpsertResult{}, fmt.Errorf("update vcs pr: %w", err)
		}
		return PRUpsertResult{
			ID:                  existing,
			StateChanged:        state != oldState,
			OldState:            oldState,
			MergeableChanged:    mergeable != oldMergeable,
			OldMergeable:        oldMergeable,
			ReviewStatusChanged: reviewStatus != oldReviewStatus,
			OldReviewStatus:     oldReviewStatus,
			CIStatusChanged:     ciStatus != oldCIStatus,
			OldCIStatus:         oldCIStatus,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return PRUpsertResult{}, fmt.Errorf("query vcs pr: %w", err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO vcs_pull_requests
			(id, repo_id, branch_id, pr_number, title, author, state, review_status, url,
			 head_branch, mergeable, ci_status, pr_created_at, pr_updated_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, repoID, branchID, prNumber, title, author, state, reviewStatus, url,
		headBranch, mergeable, ciStatus, prCreatedAt, prUpdatedAt, now)
	if err != nil {
		return PRUpsertResult{}, fmt.Errorf("insert vcs pr: %w", err)
	}
	return PRUpsertResult{ID: id, IsNew: true}, nil
}

// ListPRsForBranch returns all PRs for a branch, ordered by pr_number.
func (r *PRRepo) ListPRsForBranch(ctx context.Context, branchID string) ([]PullRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, repo_id, branch_id, pr_number, title, author, state, review_status, url,
		       head_branch, mergeable, ci_status, pr_created_at, pr_updated_at, fetched_at, viewed_at, ai_processed_at
		FROM vcs_pull_requests
		WHERE branch_id = ?
		ORDER BY pr_number ASC
	`, branchID)
	if err != nil {
		return nil, fmt.Errorf("list prs for branch: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanPRs(rows)
}

// ListPRsForContext returns all PRs for all branches of a context (via join).
func (r *PRRepo) ListPRsForContext(ctx context.Context, contextID string) ([]PullRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.repo_id, p.branch_id, p.pr_number, p.title, p.author, p.state,
		       p.review_status, p.url, p.head_branch, p.mergeable, p.ci_status,
		       p.pr_created_at, p.pr_updated_at, p.fetched_at, p.viewed_at, p.ai_processed_at
		FROM vcs_pull_requests p
		JOIN context_vcs_branches b ON b.id = p.branch_id
		WHERE b.context_id = ?
		ORDER BY p.pr_number ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list prs for context: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanPRs(rows)
}

// GetPRByRepoAndNumber returns a PR by repo + PR number.
func (r *PRRepo) GetPRByRepoAndNumber(ctx context.Context, repoID string, prNumber int) (*PullRequest, error) {
	var pr PullRequest
	err := r.db.QueryRowContext(ctx, `
		SELECT id, repo_id, branch_id, pr_number, title, author, state, review_status, url,
		       head_branch, mergeable, ci_status, pr_created_at, pr_updated_at, fetched_at, viewed_at, ai_processed_at
		FROM vcs_pull_requests WHERE repo_id = ? AND pr_number = ?
	`, repoID, prNumber).Scan(
		&pr.ID, &pr.RepoID, &pr.BranchID, &pr.PRNumber, &pr.Title, &pr.Author,
		&pr.State, &pr.ReviewStatus, &pr.URL, &pr.HeadBranch, &pr.Mergeable, &pr.CIStatus,
		&pr.PRCreatedAt, &pr.PRUpdatedAt, &pr.FetchedAt, &pr.ViewedAt, &pr.AiProcessedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vcs pr: %w", err)
	}
	return &pr, nil
}

// MarkPRViewed sets viewed_at = datetime('now') for the given PR if not already viewed.
func (r *PRRepo) MarkPRViewed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE vcs_pull_requests SET viewed_at = datetime('now') WHERE id = ? AND viewed_at IS NULL", id)
	return err
}

// MarkPRCommentViewed sets viewed_at = datetime('now') for the given PR comment if not already viewed.
func (r *PRRepo) MarkPRCommentViewed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE vcs_pr_comments SET viewed_at = datetime('now') WHERE id = ? AND viewed_at IS NULL", id)
	return err
}

// MarkBranchAllViewed marks all PRs for the given branch and all their comments as viewed.
func (r *PRRepo) MarkBranchAllViewed(ctx context.Context, branchID string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE vcs_pull_requests SET viewed_at = datetime('now')
		 WHERE branch_id = ? AND viewed_at IS NULL`, branchID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE vcs_pr_comments SET viewed_at = datetime('now')
		 WHERE pr_id IN (SELECT id FROM vcs_pull_requests WHERE branch_id = ?) AND viewed_at IS NULL`, branchID)
	return err
}

func (r *PRRepo) scanPRs(rows *sql.Rows) ([]PullRequest, error) {
	var result []PullRequest
	for rows.Next() {
		var pr PullRequest
		if err := rows.Scan(
			&pr.ID, &pr.RepoID, &pr.BranchID, &pr.PRNumber, &pr.Title, &pr.Author,
			&pr.State, &pr.ReviewStatus, &pr.URL, &pr.HeadBranch, &pr.Mergeable, &pr.CIStatus,
			&pr.PRCreatedAt, &pr.PRUpdatedAt, &pr.FetchedAt, &pr.ViewedAt, &pr.AiProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan vcs pr: %w", err)
		}
		result = append(result, pr)
	}
	return result, rows.Err()
}

// UpsertComment creates or updates a PR comment identified by (prID, externalID).
func (r *PRRepo) UpsertComment(ctx context.Context, prID, externalID string, parentID *string, commentType, author, body string, path *string, line *int, isResolved bool, inReplyToID *string, createdAt, updatedAt string) error {
	var existing string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM vcs_pr_comments WHERE pr_id = ? AND external_id = ?`,
		prID, externalID,
	).Scan(&existing)

	if err == nil {
		resolved := 0
		if isResolved {
			resolved = 1
		}
		_, err = r.db.ExecContext(ctx, `
			UPDATE vcs_pr_comments
			SET parent_id = ?, comment_type = ?, author = ?, body = ?, path = ?, line = ?,
			    is_resolved = ?, in_reply_to_id = ?, comment_updated_at = ?
			WHERE id = ?
		`, parentID, commentType, author, body, path, line, resolved, inReplyToID, updatedAt, existing)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query vcs comment: %w", err)
	}

	id := uuid.New().String()
	resolved := 0
	if isResolved {
		resolved = 1
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO vcs_pr_comments
			(id, pr_id, external_id, parent_id, comment_type, author, body, path, line,
			 is_resolved, in_reply_to_id, comment_created_at, comment_updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, prID, externalID, parentID, commentType, author, body, path, line,
		resolved, inReplyToID, createdAt, updatedAt)
	if err != nil {
		return fmt.Errorf("insert vcs comment: %w", err)
	}
	return nil
}

// ListCommentsForPR returns all comments for a PR as a flat list (no tree building).
func (r *PRRepo) ListCommentsForPR(ctx context.Context, prID string) ([]PRComment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pr_id, external_id, parent_id, comment_type, author, body, path, line,
		       is_resolved, in_reply_to_id, comment_created_at, comment_updated_at, viewed_at, ai_processed_at
		FROM vcs_pr_comments
		WHERE pr_id = ?
		ORDER BY comment_created_at ASC
	`, prID)
	if err != nil {
		return nil, fmt.Errorf("list comments for pr: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanComments(rows)
}

// ListCommentsForPRFiltered returns comments for a PR with optional resolved filter.
// When includeResolved is false, only unresolved comments are returned.
func (r *PRRepo) ListCommentsForPRFiltered(ctx context.Context, prID string, includeResolved bool) ([]PRComment, error) {
	q := `
		SELECT id, pr_id, external_id, parent_id, comment_type, author, body, path, line,
		       is_resolved, in_reply_to_id, comment_created_at, comment_updated_at, viewed_at, ai_processed_at
		FROM vcs_pr_comments
		WHERE pr_id = ?`
	if !includeResolved {
		q += " AND is_resolved = 0"
	}
	q += " ORDER BY comment_created_at ASC"

	rows, err := r.db.QueryContext(ctx, q, prID)
	if err != nil {
		return nil, fmt.Errorf("list filtered comments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanComments(rows)
}

// ListCommentsForPRs returns comments for multiple PRs in a single query.
// The result map is keyed by prID. Empty input returns an empty map.
func (r *PRRepo) ListCommentsForPRs(ctx context.Context, prIDs []string) (map[string][]PRComment, error) {
	if len(prIDs) == 0 {
		return map[string][]PRComment{}, nil
	}
	placeholders := strings.Repeat(",?", len(prIDs))[1:]
	args := make([]any, len(prIDs))
	for i, id := range prIDs {
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pr_id, external_id, parent_id, comment_type, author, body, path, line,
		       is_resolved, in_reply_to_id, comment_created_at, comment_updated_at, viewed_at, ai_processed_at
		FROM vcs_pr_comments
		WHERE pr_id IN (`+placeholders+`)
		ORDER BY comment_created_at ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("bulk list pr comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]PRComment, len(prIDs))
	for rows.Next() {
		var c PRComment
		var resolved int
		if err := rows.Scan(
			&c.ID, &c.PRID, &c.ExternalID, &c.ParentID, &c.CommentType, &c.Author, &c.Body,
			&c.Path, &c.Line, &resolved, &c.InReplyToID, &c.CommentCreatedAt, &c.CommentUpdatedAt, &c.ViewedAt, &c.AiProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan vcs comment: %w", err)
		}
		c.IsResolved = resolved == 1
		result[c.PRID] = append(result[c.PRID], c)
	}
	return result, rows.Err()
}

// BuildCommentTree takes a flat list of comments and nests replies under their parents.
// It uses in_reply_to_id (external ID of parent) to reconstruct GitHub-style threading.
func BuildCommentTree(flat []PRComment) []PRComment {
	byExtID := make(map[string]*PRComment, len(flat))
	for i := range flat {
		byExtID[flat[i].ExternalID] = &flat[i]
	}

	var roots []PRComment
	for i := range flat {
		c := &flat[i]
		if c.InReplyToID != nil && *c.InReplyToID != "" {
			if parent, ok := byExtID[*c.InReplyToID]; ok {
				parent.Replies = append(parent.Replies, *c)
				continue
			}
		}
		roots = append(roots, *c)
	}
	return roots
}

func (r *PRRepo) scanComments(rows *sql.Rows) ([]PRComment, error) {
	var result []PRComment
	for rows.Next() {
		var c PRComment
		var resolved int
		if err := rows.Scan(
			&c.ID, &c.PRID, &c.ExternalID, &c.ParentID, &c.CommentType, &c.Author, &c.Body,
			&c.Path, &c.Line, &resolved, &c.InReplyToID, &c.CommentCreatedAt, &c.CommentUpdatedAt, &c.ViewedAt, &c.AiProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan vcs comment: %w", err)
		}
		c.IsResolved = resolved == 1
		result = append(result, c)
	}
	return result, rows.Err()
}
