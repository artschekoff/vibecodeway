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
	Number     int
	Title      string
	Author     string
	State      string
	ReviewStatus string
	URL        string
	HeadBranch string
	// Mergeable is the GitHub mergeability status: "MERGEABLE", "CONFLICTING", or "UNKNOWN".
	Mergeable  string
	CreatedAt  string
	UpdatedAt  string
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

	// ListRepoBranches returns branch names for the given repository (up to 100).
	ListRepoBranches(ctx context.Context, owner, repo string) ([]string, error)

	// StartDeviceAuth initiates the device-code OAuth flow and returns the code info.
	StartDeviceAuth(ctx context.Context) (*DeviceCodeInfo, error)

	// CompleteDeviceAuth polls until the user completes the device-code flow,
	// persists the token for CLI use, and returns the authenticated user info.
	CompleteDeviceAuth(ctx context.Context, deviceCode string, interval int) (AuthResult, error)

	// GetAuthUser returns the currently authenticated username, or an error if not authenticated.
	GetAuthUser(ctx context.Context) (string, error)
}
