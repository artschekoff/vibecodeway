package store

import (
	"context"
	"fmt"
)

// BoardBlocker is a blocker_identified event enriched with its context key.
type BoardBlocker struct {
	EventID    string `json:"eventId"`
	ContextID  string `json:"contextId"`
	ContextKey string `json:"contextKey"`
	Author     string `json:"author"`
	Summary    string `json:"summary"`
	Timestamp  string `json:"timestamp"`
	CreatedAt  string `json:"createdAt"`
}

// BoardAssignment is a PersonAssignment enriched with its context key.
type BoardAssignment struct {
	ID              string  `json:"id"`
	ContextID       string  `json:"contextId"`
	ContextKey      string  `json:"contextKey"`
	Name            string  `json:"name"`
	Body            string  `json:"body"`
	GeneratedAt     *string `json:"generatedAt,omitempty"`
	AnalyzedThrough *string `json:"analyzedThrough,omitempty"`
	IsOutdated      bool    `json:"isOutdated"`
	UpdatedAt       string  `json:"updatedAt"`
}

// BoardPR is an open pull request enriched with context and repo info.
type BoardPR struct {
	ID           string `json:"id"`
	ContextID    string `json:"contextId"`
	ContextKey   string `json:"contextKey"`
	RepoName     string `json:"repoName"`
	PRNumber     int    `json:"prNumber"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	HeadBranch   string `json:"headBranch"`
	ReviewStatus string `json:"reviewStatus"`
	CIStatus     string `json:"ciStatus"`
	Mergeable    string `json:"mergeable"`
	IsDraft      bool   `json:"isDraft"`
	State        string `json:"state"`
	PRUpdatedAt  string `json:"prUpdatedAt"`
}

// BoardJiraAssignment is one Jira issue assigned to the current user, persisted
// as a personal to-do item. Seen is false until the user has viewed it in the
// Activity Feed.
type BoardJiraAssignment struct {
	Key       string `json:"key"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	IssueType string `json:"issueType"`
	Priority  string `json:"priority"`
	Created   string `json:"created"`
	Seen      bool   `json:"seen"`
	FetchedAt string `json:"fetchedAt"`
	// ContextID/ContextKey are set when this assignment's Jira key already belongs
	// to a context (so the board can show/open the linked context). Nil otherwise.
	ContextID  *string `json:"contextId,omitempty"`
	ContextKey *string `json:"contextKey,omitempty"`
}

// MyBoardData aggregates all sections of the My Board page.
type MyBoardData struct {
	Blockers             []BoardBlocker        `json:"blockers"`
	MyAssignments        []BoardAssignment     `json:"myAssignments"`
	MyOpenPRs            []BoardPR             `json:"myOpenPRs"`
	MyAssignedJiraIssues []BoardJiraAssignment `json:"myAssignedJiraIssues"`
}

// ListBoardBlockers returns recent blocker_identified events across all contexts, newest first.
// ponytail: returns last 50 — no pagination until needed
func (s *Store) ListBoardBlockers(ctx context.Context) ([]BoardBlocker, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.context_id, c.key, e.author, e.summary, e.timestamp, e.created_at
		FROM event_log e
		JOIN contexts c ON c.id = e.context_id
		WHERE e.type = 'blocker_identified'
		ORDER BY e.created_at DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("list board blockers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BoardBlocker
	for rows.Next() {
		var b BoardBlocker
		if err := rows.Scan(&b.EventID, &b.ContextID, &b.ContextKey, &b.Author, &b.Summary, &b.Timestamp, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan board blocker: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListBoardAssignments returns all PersonAssignments for the given name across all contexts.
// If name is empty, returns all assignments (useful when user name is not configured).
func (s *Store) ListBoardAssignments(ctx context.Context, name string) ([]BoardAssignment, error) {
	query := `
		SELECT pa.id, pa.context_id, c.key, pa.name, pa.body,
		       pa.generated_at, pa.analyzed_through, pa.is_outdated, pa.updated_at
		FROM context_person_assignments pa
		JOIN contexts c ON c.id = pa.context_id
		WHERE (? = '' OR pa.name = ?)
		ORDER BY pa.is_outdated DESC, pa.updated_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, name, name)
	if err != nil {
		return nil, fmt.Errorf("list board assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BoardAssignment
	for rows.Next() {
		var a BoardAssignment
		var isOutdatedInt int
		if err := rows.Scan(
			&a.ID, &a.ContextID, &a.ContextKey, &a.Name, &a.Body,
			&a.GeneratedAt, &a.AnalyzedThrough, &isOutdatedInt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan board assignment: %w", err)
		}
		a.IsOutdated = isOutdatedInt == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListBoardPRsByAuthor returns open pull requests authored by the given user across all contexts.
func (s *Store) ListBoardPRsByAuthor(ctx context.Context, author string) ([]BoardPR, error) {
	if author == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, b.context_id, c.key, r.name,
		       p.pr_number, p.title, p.url, p.head_branch,
		       p.review_status, p.ci_status, p.mergeable, p.is_draft, p.state, p.pr_updated_at
		FROM vcs_pull_requests p
		JOIN context_vcs_branches b ON b.id = p.branch_id
		JOIN contexts c ON c.id = b.context_id
		JOIN vcs_repos r ON r.id = p.repo_id
		WHERE p.author = ? AND p.state = 'open'
		ORDER BY p.pr_updated_at DESC
	`, author)
	if err != nil {
		return nil, fmt.Errorf("list board prs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BoardPR
	for rows.Next() {
		var p BoardPR
		var isDraftInt int
		if err := rows.Scan(
			&p.ID, &p.ContextID, &p.ContextKey, &p.RepoName,
			&p.PRNumber, &p.Title, &p.URL, &p.HeadBranch,
			&p.ReviewStatus, &p.CIStatus, &p.Mergeable, &isDraftInt, &p.State, &p.PRUpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan board pr: %w", err)
		}
		p.IsDraft = isDraftInt == 1
		out = append(out, p)
	}
	return out, rows.Err()
}
