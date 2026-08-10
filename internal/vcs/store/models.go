// Package vcsstore provides persistence models and repositories for VCS (GitHub, GitLab, Linear) data.
package vcsstore

// VCSAuth holds authentication state for one VCS platform.
type VCSAuth struct {
	ID        string `json:"id"`
	Platform  string `json:"platform"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatarUrl"`
	Token     string `json:"-"` // never serialized to the frontend
	UpdatedAt string `json:"updatedAt"`
}

// VCSRepo is a tracked repository. It is global (not per-context) and shared across contexts.
type VCSRepo struct {
	ID                string `json:"id"`
	Platform          string `json:"platform"`
	Owner             string `json:"owner"`
	Name              string `json:"name"`
	LocalPath         string `json:"localPath"`
	Label             string `json:"label"`
	Description       string `json:"description"`
	OnCheckoutCommand string `json:"onCheckoutCommand"`
	CreatedAt         string `json:"createdAt"`
}

// FullName returns "owner/name".
func (r VCSRepo) FullName() string { return r.Owner + "/" + r.Name }

// VCSRepoPath is one local filesystem path for a tracked repo.
// A repo can have multiple paths (e.g. different clones for different teams).
type VCSRepoPath struct {
	ID        string `json:"id"`
	RepoID    string `json:"repoId"`
	Path      string `json:"path"`
	CreatedAt string `json:"createdAt"`
}

// ContextVCSBranch links a specific branch of a VCSRepo to a context.
type ContextVCSBranch struct {
	ID        string   `json:"id"`
	ContextID string   `json:"contextId"`
	RepoID    string   `json:"repoId"`
	Branch    string   `json:"branch"`
	Label     string   `json:"label"`
	CreatedAt string   `json:"createdAt"`
	Repo      *VCSRepo `json:"repo,omitempty"`
}

// PullRequest represents a PR/MR stored for a branch.
type PullRequest struct {
	ID           string `json:"id"`
	RepoID       string `json:"repoId"`
	BranchID     string `json:"branchId"`
	PRNumber     int    `json:"prNumber"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	State        string `json:"state"`
	IsDraft      bool   `json:"isDraft"`
	ReviewStatus string `json:"reviewStatus"`
	URL          string `json:"url"`
	HeadBranch   string `json:"headBranch"`
	// Mergeable is the GitHub mergeability status: "MERGEABLE", "CONFLICTING", or "UNKNOWN".
	Mergeable string `json:"mergeable"`
	// MergeStateStatus is GitHub's merge-state; "BEHIND" means the "Update branch" action is available.
	MergeStateStatus string `json:"mergeStateStatus"`
	// CIStatus is the aggregated status-check rollup: "PENDING", "SUCCESS", "FAILURE", or "".
	CIStatus      string      `json:"ciStatus"`
	PRCreatedAt   string      `json:"prCreatedAt"`
	PRUpdatedAt   string      `json:"prUpdatedAt"`
	FetchedAt     string      `json:"fetchedAt"`
	ViewedAt      *string     `json:"viewedAt"`
	AiProcessedAt *string     `json:"aiProcessedAt"`
	MessageID     *string     `json:"messageId,omitempty"`
	Comments      []PRComment `json:"comments,omitempty"`
}

// PRComment represents one comment on a PR: a conversation comment, an inline review comment,
// or a review body. Parent/reply threading is expressed via ParentID.
type PRComment struct {
	ID               string      `json:"id"`
	PRID             string      `json:"prId"`
	ExternalID       string      `json:"externalId"`
	ParentID         *string     `json:"parentId,omitempty"`
	CommentType      string      `json:"commentType"`
	Author           string      `json:"author"`
	Body             string      `json:"body"`
	Path             *string     `json:"path,omitempty"`
	Line             *int        `json:"line,omitempty"`
	IsResolved       bool        `json:"isResolved"`
	InReplyToID      *string     `json:"inReplyToId,omitempty"`
	CommentCreatedAt string      `json:"commentCreatedAt"`
	CommentUpdatedAt string      `json:"commentUpdatedAt"`
	ViewedAt         *string     `json:"viewedAt"`
	AiProcessedAt    *string     `json:"aiProcessedAt"`
	MessageID        *string     `json:"messageId,omitempty"`
	Replies          []PRComment `json:"replies,omitempty"`
}

// ReviewRequest represents a PR where this user's review has been requested.
type ReviewRequest struct {
	ID           string `json:"id"`
	RepoID       string `json:"repoId"`
	PRNumber     int    `json:"prNumber"`
	PRTitle      string `json:"prTitle"`
	PRAuthor     string `json:"prAuthor"`
	PRURL        string `json:"prUrl"`
	ReviewStatus string `json:"reviewStatus"`
	HeadBranch   string `json:"headBranch"`
	RequestedAt  string `json:"requestedAt"`
	FetchedAt    string `json:"fetchedAt"`
	// Mergeable is the GitHub mergeability status ("MERGEABLE", "CONFLICTING",
	// "UNKNOWN"), refreshed every sync. Empty when never fetched.
	Mergeable string `json:"mergeable"`
	// ReviewedAt is set when the item disappears from GitHub's review-requested list
	// (user submitted a review). NULL = still actively pending.
	ReviewedAt *string `json:"reviewedAt,omitempty"`
	// DismissedAt is set when the user manually dismisses a "waiting for author" item.
	DismissedAt *string            `json:"dismissedAt,omitempty"`
	ViewedAt    *string            `json:"viewedAt,omitempty"`
	Reviewers   []ReviewerDecision `json:"reviewers,omitempty"`
	// Kind labels how this PR entered the list: "requested" (someone asked for my
	// review) or "authored" (my own open PR shown for status tracking).
	Kind string `json:"kind"`
	// CIStatus is the aggregated status-check rollup: "", PENDING, SUCCESS, FAILURE.
	CIStatus string `json:"ciStatus"`
}

// ReviewerDecision is one reviewer's latest decision on a PR.
type ReviewerDecision struct {
	Login       string `json:"login"`
	State       string `json:"state"` // approved | changes_requested | commented
	SubmittedAt string `json:"submittedAt"`
}

// VCSPlatformInfo describes a platform with its enabled flag.
type VCSPlatformInfo struct {
	Platform string `json:"platform"`
	Label    string `json:"label"`
	Enabled  bool   `json:"enabled"`
}

// VCSWorkspace tracks a git worktree checked out for a specific branch.
// Status is "active" or "should_delete".
type VCSWorkspace struct {
	ID           string `json:"id"`
	RepoPathID   string `json:"repoPathId"`
	RepoID       string `json:"repoId"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktreePath"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}
