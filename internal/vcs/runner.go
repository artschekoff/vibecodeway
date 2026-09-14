// Package vcs provides the platform-agnostic VCS runner interface and platform constants.
// GitHub is the first and currently only enabled implementation.
// To add GitLab or Linear: implement VCSRunner, register it in vcsservice.NewVCSProvider,
// and flip IsPlatformEnabled.
package vcs

import "context"

// Platform identifies a VCS hosting platform.
type Platform string

const (
	PlatformGitHub Platform = "github"
	PlatformGitLab Platform = "gitlab" // future
	PlatformLinear Platform = "linear" // future
)

// KnownPlatforms returns all known platforms in display order.
func KnownPlatforms() []Platform {
	return []Platform{PlatformGitHub, PlatformGitLab, PlatformLinear}
}

// IsPlatformEnabled reports whether a platform has a runner implementation available.
func IsPlatformEnabled(p Platform) bool {
	return p == PlatformGitHub
}

// PlatformLabel returns the human-readable label for a platform.
func PlatformLabel(p Platform) string {
	switch p {
	case PlatformGitHub:
		return "GitHub"
	case PlatformGitLab:
		return "GitLab"
	case PlatformLinear:
		return "Linear"
	default:
		return string(p)
	}
}

// PRData contains the fields returned by ListPRsForBranch and ListReviewRequests.
type PRData struct {
	Number       int
	Title        string
	Author       string
	State        string
	ReviewStatus string
	URL          string
	HeadBranch   string
	IsDraft      bool
	// Mergeable is the GitHub mergeability status: "MERGEABLE", "CONFLICTING", or "UNKNOWN".
	Mergeable string
	// MergeStateStatus is GitHub's merge-state: "BEHIND" means the head branch is out of
	// date with base (the "Update branch" button is available). Other values: "CLEAN",
	// "BLOCKED", "DIRTY", "DRAFT", "HAS_HOOKS", "UNSTABLE", "UNKNOWN".
	MergeStateStatus string
	// CIStatus is the aggregated status-check rollup: "PENDING", "SUCCESS", "FAILURE", or "".
	CIStatus  string
	CreatedAt string
	UpdatedAt string
	// MergedAt is the real merge timestamp (RFC3339, gh's native format),
	// only populated by GetPRByNumber's single-PR `gh pr view` fetch (list
	// queries don't request it). Empty when the PR isn't merged or the field
	// wasn't fetched.
	MergedAt  string
	Reviewers []ReviewerDecision `json:"reviewers,omitempty"`
	// MyReviewState is the authenticated user's own standing review decision:
	// "", "approved", "changes_requested" or "commented". nil means the PR's
	// reviews payload was not fetched for this call, so persisting layers must
	// leave any stored value alone instead of resetting it.
	MyReviewState *string
}

// ReviewerDecision is one reviewer's latest effective decision on a PR.
type ReviewerDecision struct {
	Login       string `json:"login"`
	State       string `json:"state"` // approved | changes_requested | commented
	SubmittedAt string `json:"submittedAt"`
}

// CommentData contains a single comment fetched from a PR.
type CommentData struct {
	ExternalID  string
	CommentType string // "conversation" | "review" | "review_comment"
	Author      string
	Body        string
	Path        *string
	Line        *int
	IsResolved  bool
	InReplyToID *string // external ID of the parent comment (for threading)
	CreatedAt   string
	UpdatedAt   string
}

// AuthResult is returned by AuthLogin.
type AuthResult struct {
	Username  string
	AvatarURL string
	Token     string
}

// DeviceCodeInfo holds the information returned when starting a device-code OAuth flow.
// The frontend displays UserCode and VerificationURI to the user.
type DeviceCodeInfo struct {
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	DeviceCode      string `json:"deviceCode"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}

// FetchAllPRLimit is the result cap requested by ListPRsForBranch when branch == ""
// (fetch-all mode, shared across every branch tracked in a repo and filtered locally
// by HeadBranch). Exported so the service-layer cache can detect a batch that hit this
// cap — meaning older PRs may have been left out — without duplicating the number.
const FetchAllPRLimit = 200

// VCSRunner is the interface all VCS platform runners must implement.
// Add a new runner in its own sub-package and register it with vcsservice.NewVCSProvider.
type VCSRunner interface {
	// Platform returns the platform this runner handles.
	Platform() Platform

	// ListPRsForBranch fetches open/recent PRs whose head branch matches the given branch name.
	ListPRsForBranch(ctx context.Context, owner, repo, branch string) ([]PRData, error)

	// GetPRComments fetches all comments (conversation + review) for a single PR.
	GetPRComments(ctx context.Context, owner, repo string, prNumber int) ([]CommentData, error)

	// ListReviewRequests fetches PRs where the authenticated user's review is requested.
	ListReviewRequests(ctx context.Context, owner, repo string) ([]PRData, error)

	// ListChangesRequestedByUser fetches open PRs where the authenticated user submitted
	// a "changes requested" review. These remain actionable until the author addresses them.
	ListChangesRequestedByUser(ctx context.Context, owner, repo string) ([]PRData, error)

	// ListAuthoredPRs fetches the given user's open authored PRs (login "" = @me),
	// with reviewer decisions, CI rollup, and mergeability populated.
	ListAuthoredPRs(ctx context.Context, owner, repo, login string) ([]PRData, error)

	// ListRepoBranches returns branch names for the given repository (up to 100).
	ListRepoBranches(ctx context.Context, owner, repo string) ([]string, error)

	// StartDeviceAuth initiates the device-code OAuth flow and returns the code info.
	StartDeviceAuth(ctx context.Context) (*DeviceCodeInfo, error)

	// CompleteDeviceAuth polls until the user completes the device-code flow,
	// persists the token for CLI use, and returns the authenticated user info.
	CompleteDeviceAuth(ctx context.Context, deviceCode string, interval int) (AuthResult, error)

	// GetAuthUser returns the currently authenticated username, or an error if not authenticated.
	GetAuthUser(ctx context.Context) (string, error)

	// SetToken stores the OAuth token on the runner so it is used for all subsequent API calls.
	// This avoids depending on the gh CLI keychain for authentication.
	SetToken(token string)

	// GetPRByNumber fetches a single PR by its number from the VCS platform.
	// Returns the PR metadata including the head branch name.
	// Used when the user attaches a PR URL directly to a context.
	GetPRByNumber(ctx context.Context, owner, repo string, prNumber int) (*PRData, error)

	// UpdatePRBranch merges the base branch into the PR's head branch on the platform
	// (GitHub's "Update branch" action). Only meaningful when the PR is behind its base.
	UpdatePRBranch(ctx context.Context, owner, repo string, prNumber int) error

	// GetRepoDescription fetches the human-readable description of a repository.
	// Returns ("", nil) when the repo has no description. Non-fatal errors return "".
	GetRepoDescription(ctx context.Context, owner, repo string) (string, error)
}
