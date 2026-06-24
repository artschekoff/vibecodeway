// Package github provides a VCSRunner implementation backed by the gh CLI.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/artschekoff/context-consolidator/internal/vcs"
	"go.uber.org/zap"
)

// githubOAuthClientID is the public OAuth App client ID used for the device flow.
// This is the well-known GitHub CLI client ID (open source, public).
const githubOAuthClientID = "178c6fc778ccc68e1d6a"

// knownExtraDirs mirrors the pattern used across this codebase for resolving
// binaries in macOS GUI app environments that inherit a minimal PATH.
var knownExtraDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
}

// Runner implements vcs.VCSRunner using the gh CLI.
type Runner struct {
	binPath    string
	timeout    time.Duration
	httpClient *http.Client
	logger     *zap.Logger

	mu    sync.RWMutex
	token string // OAuth token; when set, passed as GH_TOKEN to all gh CLI invocations
}

// NewRunner creates a Runner that resolves the gh binary from PATH and known dirs.
func NewRunner(logger *zap.Logger) *Runner {
	return &Runner{
		binPath:    resolveBinPath("gh", knownExtraDirs),
		timeout:    60 * time.Second,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		logger:     logger,
	}
}

// SetToken stores the OAuth token so it is injected as GH_TOKEN into every gh CLI call,
// removing the dependency on gh's own keychain.
func (r *Runner) SetToken(token string) {
	r.mu.Lock()
	r.token = token
	r.mu.Unlock()
}

// Platform implements vcs.VCSRunner.
func (r *Runner) Platform() vcs.Platform { return vcs.PlatformGitHub }

// StartDeviceAuth initiates the GitHub OAuth device-code flow via HTTP.
func (r *Runner) StartDeviceAuth(ctx context.Context) (*vcs.DeviceCodeInfo, error) {
	data := url.Values{
		"client_id": {githubOAuthClientID},
		"scope":     {"repo,read:org"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/device/code", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse device code response: %w", err)
	}

	r.logger.Info("github device auth started",
		zap.String("user_code", result.UserCode),
		zap.Int("interval", result.Interval),
		zap.Int("expires_in", result.ExpiresIn),
	)

	return &vcs.DeviceCodeInfo{
		UserCode:        result.UserCode,
		VerificationURI: result.VerificationURI,
		DeviceCode:      result.DeviceCode,
		ExpiresIn:       result.ExpiresIn,
		Interval:        result.Interval,
	}, nil
}

// CompleteDeviceAuth polls GitHub until the user completes the device-code flow,
// then persists the token via `gh auth login --with-token` and returns user info.
func (r *Runner) CompleteDeviceAuth(ctx context.Context, deviceCode string, interval int) (vcs.AuthResult, error) {
	if interval < 5 {
		interval = 5
	}
	r.logger.Info("github device auth: polling started", zap.Int("interval_s", interval))

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("github device auth: context cancelled", zap.Error(ctx.Err()))
			return vcs.AuthResult{}, ctx.Err()
		case <-ticker.C:
		}

		token, err := r.pollAccessToken(ctx, deviceCode)
		if err != nil {
			return vcs.AuthResult{}, err
		}
		if token == "" {
			continue // authorization_pending, keep polling
		}

		r.logger.Info("github device auth: token received, fetching user")

		// Fetch username directly via the GitHub API using the token — this works
		// regardless of whether gh CLI is installed or accessible.
		username, err := r.getUserWithToken(ctx, token)
		if err != nil {
			return vcs.AuthResult{}, fmt.Errorf("get user after auth: %w", err)
		}

		r.logger.Info("github device auth: user resolved", zap.String("username", username))

		// Immediately set the token on the runner so subsequent gh CLI calls work
		// without waiting for keychain persistence.
		r.SetToken(token)

		// Best-effort: persist token into gh CLI's keychain in the background.
		// This is not required for the app to work (we use GH_TOKEN env var instead),
		// but keeps the gh CLI usable in the terminal after connecting via the app.
		go func() {
			persistCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := r.persistToken(persistCtx, token); err != nil {
				r.logger.Warn("gh auth login --with-token failed (non-fatal)", zap.Error(err))
			}
		}()

		return vcs.AuthResult{Username: username, Token: token}, nil
	}
}

// pollAccessToken makes a single poll request to the GitHub token endpoint.
// Returns the access token when authorized, empty string if still pending, or an error.
func (r *Runner) pollAccessToken(ctx context.Context, deviceCode string) (string, error) {
	data := url.Values{
		"client_id":   {githubOAuthClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}

	r.logger.Info("github poll response", zap.String("error_field", result.Error), zap.Bool("has_token", result.AccessToken != ""))

	switch result.Error {
	case "":
		return result.AccessToken, nil
	case "authorization_pending":
		return "", nil
	case "slow_down":
		return "", nil
	case "expired_token":
		return "", fmt.Errorf("device code expired, please try again")
	case "access_denied":
		return "", fmt.Errorf("access denied by user")
	default:
		return "", fmt.Errorf("unexpected error from GitHub: %s", result.Error)
	}
}

// getUserWithToken fetches the authenticated user's login directly from the GitHub REST API
// using a bearer token. This avoids any dependency on the gh CLI being installed or configured.
func (r *Runner) getUserWithToken(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", fmt.Errorf("build user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("user request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user request returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse user response: %w", err)
	}
	if result.Login == "" {
		return "", fmt.Errorf("empty login in user response")
	}
	return result.Login, nil
}

// persistToken feeds the token to `gh auth login --with-token` so subsequent gh commands work.
func (r *Runner) persistToken(ctx context.Context, token string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "auth", "login", "--with-token") //nolint:gosec
	cmd.Stdin = strings.NewReader(token)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh auth login --with-token: %s (%w)", stderr.String(), err)
	}
	return nil
}

// GetAuthUser returns the username for the current gh auth session.
func (r *Runner) GetAuthUser(ctx context.Context) (string, error) {
	type ghUser struct {
		Login string `json:"login"`
	}
	out, err := r.run(ctx, []string{"api", "user"})
	if err != nil {
		return "", fmt.Errorf("gh api user: %w", err)
	}
	var u ghUser
	if err := json.Unmarshal(out, &u); err != nil {
		return "", fmt.Errorf("parsing gh user: %w", err)
	}
	return u.Login, nil
}

// ListPRsForBranch fetches PRs for the given repo. When branch is non-empty only open PRs whose
// head branch matches are returned; when branch is empty all open PRs for the repo are returned.
func (r *Runner) ListPRsForBranch(ctx context.Context, owner, repo, branch string) ([]vcs.PRData, error) {
	args := []string{
		"pr", "list",
		"--repo", owner + "/" + repo,
		"--state", "all",
		"--json", "number,title,author,state,isDraft,reviewDecision,url,headRefName,mergeable,statusCheckRollup,createdAt,updatedAt",
		"--limit", "50",
	}
	if branch != "" {
		args = append(args, "--head", branch)
	}
	r.logger.Info("gh pr list",
		zap.String("repo", owner+"/"+repo),
		zap.String("branch", branch),
	)
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}
	return parseGHPRList(out, "pr list")
}

// GetPRComments fetches all comments for a PR: inline review comments, review bodies,
// and conversation (issue) comments. All three sources are merged and returned flat.
func (r *Runner) GetPRComments(ctx context.Context, owner, repo string, prNumber int) ([]vcs.CommentData, error) {
	repoPath := owner + "/" + repo
	var allComments []vcs.CommentData

	// 1. Inline review comments (pull request review comments)
	reviewComments, err := r.fetchReviewComments(ctx, repoPath, prNumber)
	if err != nil {
		r.logger.Warn("gh: fetch review comments failed", zap.Int("pr", prNumber), zap.Error(err))
	}
	allComments = append(allComments, reviewComments...)

	// 2. Conversation comments (issue comments on the PR thread)
	issueComments, err := r.fetchIssueComments(ctx, repoPath, prNumber)
	if err != nil {
		r.logger.Warn("gh: fetch issue comments failed", zap.Int("pr", prNumber), zap.Error(err))
	}
	allComments = append(allComments, issueComments...)

	// 3. Reviews (each review has a body + a state we use for is_resolved)
	reviews, err := r.fetchReviews(ctx, repoPath, prNumber)
	if err != nil {
		r.logger.Warn("gh: fetch reviews failed", zap.Int("pr", prNumber), zap.Error(err))
	}
	allComments = append(allComments, reviews...)

	return allComments, nil
}

// GetPRByNumber fetches a single PR by its number using `gh pr view`.
func (r *Runner) GetPRByNumber(ctx context.Context, owner, repo string, prNumber int) (*vcs.PRData, error) {
	r.logger.Info("gh pr view", zap.String("repo", owner+"/"+repo), zap.Int("pr", prNumber))
	out, err := r.run(ctx, []string{
		"pr", "view", strconv.Itoa(prNumber),
		"--repo", owner + "/" + repo,
		"--json", "number,title,author,state,isDraft,reviewDecision,url,headRefName,mergeable,statusCheckRollup,createdAt,updatedAt",
	})
	if err != nil {
		return nil, fmt.Errorf("gh pr view %d: %w", prNumber, err)
	}
	pd, err := parseGHPRSingle(out)
	if err != nil {
		return nil, err
	}
	return &pd, nil
}

// ghPR is the JSON shape returned by `gh pr list --json ...` for PR list queries.
type ghPR struct {
	Number         int            `json:"number"`
	Title          string         `json:"title"`
	Author         struct{ Login string } `json:"author"`
	State          string         `json:"state"`
	IsDraft        bool           `json:"isDraft"`
	ReviewDecision string         `json:"reviewDecision"`
	URL            string         `json:"url"`
	HeadRefName    string         `json:"headRefName"`
	Mergeable      string         `json:"mergeable"`
	StatusCheckRollup []struct {
		State string `json:"state"`
	} `json:"statusCheckRollup"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// parseGHPRList unmarshals the JSON output of `gh pr list --json ...` into []vcs.PRData.
func parseGHPRList(data []byte, errLabel string) ([]vcs.PRData, error) {
	var ghPRs []ghPR
	if err := json.Unmarshal(data, &ghPRs); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", errLabel, err)
	}
	result := make([]vcs.PRData, 0, len(ghPRs))
	for _, p := range ghPRs {
		states := make([]string, len(p.StatusCheckRollup))
		for i, c := range p.StatusCheckRollup {
			states[i] = c.State
		}
		result = append(result, vcs.PRData{
			Number:       p.Number,
			Title:        p.Title,
			Author:       p.Author.Login,
			State:        strings.ToLower(p.State),
			IsDraft:      p.IsDraft,
			ReviewStatus: mapReviewDecision(p.ReviewDecision),
			URL:          p.URL,
			HeadBranch:   p.HeadRefName,
			Mergeable:    p.Mergeable,
			CIStatus:     aggregateCIStatus(states),
			CreatedAt:    p.CreatedAt,
			UpdatedAt:    p.UpdatedAt,
		})
	}
	return result, nil
}

// parseGHPRSingle unmarshals the JSON output of `gh pr view <number> --json ...` into vcs.PRData.
// Unlike parseGHPRList, this parses a single JSON object (not an array).
func parseGHPRSingle(data []byte) (vcs.PRData, error) {
	var p ghPR
	if err := json.Unmarshal(data, &p); err != nil {
		return vcs.PRData{}, fmt.Errorf("parsing pr view: %w", err)
	}
	states := make([]string, len(p.StatusCheckRollup))
	for i, c := range p.StatusCheckRollup {
		states[i] = c.State
	}
	return vcs.PRData{
		Number:       p.Number,
		Title:        p.Title,
		Author:       p.Author.Login,
		State:        strings.ToLower(p.State),
		IsDraft:      p.IsDraft,
		ReviewStatus: mapReviewDecision(p.ReviewDecision),
		URL:          p.URL,
		HeadBranch:   p.HeadRefName,
		Mergeable:    p.Mergeable,
		CIStatus:     aggregateCIStatus(states),
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}, nil
}

// ListReviewRequests fetches PRs where the authenticated user's review is requested.
func (r *Runner) ListReviewRequests(ctx context.Context, owner, repo string) ([]vcs.PRData, error) {
	out, err := r.run(ctx, []string{
		"pr", "list",
		"--repo", owner + "/" + repo,
		"--search", "review-requested:@me",
		"--state", "open",
		"--json", "number,title,author,state,reviewDecision,url,headRefName,mergeable,statusCheckRollup,createdAt,updatedAt",
		"--limit", "100",
	})
	if err != nil {
		return nil, fmt.Errorf("gh pr list review-requested: %w", err)
	}
	return parseGHPRList(out, "review requests")
}

// ListChangesRequestedByUser fetches open PRs where the authenticated user submitted
// a "changes requested" review. These remain actionable until the author addresses them.
func (r *Runner) ListChangesRequestedByUser(ctx context.Context, owner, repo string) ([]vcs.PRData, error) {
	out, err := r.run(ctx, []string{
		"pr", "list",
		"--repo", owner + "/" + repo,
		"--search", "reviewed-by:@me review:changes_requested",
		"--state", "open",
		"--json", "number,title,author,state,reviewDecision,url,headRefName,mergeable,statusCheckRollup,createdAt,updatedAt",
		"--limit", "100",
	})
	if err != nil {
		return nil, fmt.Errorf("gh pr list changes-requested-by-me: %w", err)
	}
	return parseGHPRList(out, "changes requested PRs")
}

// GetRepoDescription fetches the description field from `gh repo view`.
func (r *Runner) GetRepoDescription(ctx context.Context, owner, repo string) (string, error) {
	out, err := r.run(ctx, []string{"repo", "view", owner + "/" + repo, "--json", "description"})
	if err != nil {
		return "", fmt.Errorf("gh repo view description: %w", err)
	}
	var result struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parsing repo description: %w", err)
	}
	return result.Description, nil
}

// ListRepoBranches returns up to 100 branch names for the given repository.
func (r *Runner) ListRepoBranches(ctx context.Context, owner, repo string) ([]string, error) {
	out, err := r.run(ctx, []string{
		"api", fmt.Sprintf("repos/%s/%s/branches", owner, repo),
		"--paginate",
		"--jq", ".[].name",
	})
	if err != nil {
		return nil, fmt.Errorf("gh api branches: %w", err)
	}

	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.Trim(line, `" `)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// fetchReviewComments fetches inline PR review thread comments via the GitHub API.
func (r *Runner) fetchReviewComments(ctx context.Context, repoPath string, prNumber int) ([]vcs.CommentData, error) {
	type ghComment struct {
		ID          int64  `json:"id"`
		Body        string `json:"body"`
		User        struct{ Login string } `json:"user"`
		Path        string `json:"path"`
		Line        *int   `json:"line"`
		OriginalLine *int  `json:"original_line"`
		InReplyToID *int64 `json:"in_reply_to_id"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		// subject_type will be "line" for inline comments
	}

	out, err := r.apiPaginated(ctx, fmt.Sprintf("repos/%s/pulls/%d/comments?per_page=100", repoPath, prNumber))
	if err != nil {
		return nil, err
	}

	var ghComments []ghComment
	if err := json.Unmarshal(out, &ghComments); err != nil {
		return nil, fmt.Errorf("parse review comments: %w", err)
	}

	result := make([]vcs.CommentData, 0, len(ghComments))
	for _, c := range ghComments {
		extID := fmt.Sprintf("review_comment_%d", c.ID)
		var inReplyTo *string
		if c.InReplyToID != nil {
			s := fmt.Sprintf("review_comment_%d", *c.InReplyToID)
			inReplyTo = &s
		}
		line := c.Line
		if line == nil {
			line = c.OriginalLine
		}
		path := c.Path
		result = append(result, vcs.CommentData{
			ExternalID:  extID,
			CommentType: "review_comment",
			Author:      c.User.Login,
			Body:        c.Body,
			Path:        &path,
			Line:        line,
			IsResolved:  false, // resolved state comes from the thread, not the comment
			InReplyToID: inReplyTo,
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
		})
	}
	return result, nil
}

// fetchIssueComments fetches conversation comments on the PR thread.
func (r *Runner) fetchIssueComments(ctx context.Context, repoPath string, prNumber int) ([]vcs.CommentData, error) {
	type ghComment struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		User      struct{ Login string } `json:"user"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	out, err := r.apiPaginated(ctx, fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", repoPath, prNumber))
	if err != nil {
		return nil, err
	}

	var ghComments []ghComment
	if err := json.Unmarshal(out, &ghComments); err != nil {
		return nil, fmt.Errorf("parse issue comments: %w", err)
	}

	result := make([]vcs.CommentData, 0, len(ghComments))
	for _, c := range ghComments {
		extID := fmt.Sprintf("issue_comment_%d", c.ID)
		result = append(result, vcs.CommentData{
			ExternalID:  extID,
			CommentType: "conversation",
			Author:      c.User.Login,
			Body:        c.Body,
			IsResolved:  false,
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
		})
	}
	return result, nil
}

// fetchReviews fetches PR reviews (submitted review bodies).
func (r *Runner) fetchReviews(ctx context.Context, repoPath string, prNumber int) ([]vcs.CommentData, error) {
	type ghReview struct {
		ID          int64  `json:"id"`
		Body        string `json:"body"`
		User        struct{ Login string } `json:"user"`
		State       string `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
		SubmittedAt string `json:"submitted_at"`
	}

	out, err := r.apiPaginated(ctx, fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=100", repoPath, prNumber))
	if err != nil {
		return nil, err
	}

	var ghReviews []ghReview
	if err := json.Unmarshal(out, &ghReviews); err != nil {
		return nil, fmt.Errorf("parse reviews: %w", err)
	}

	result := make([]vcs.CommentData, 0, len(ghReviews))
	for _, rv := range ghReviews {
		if strings.TrimSpace(rv.Body) == "" {
			continue // skip reviews with no body text
		}
		extID := fmt.Sprintf("review_%d", rv.ID)
		resolved := rv.State == "APPROVED" || rv.State == "DISMISSED"
		result = append(result, vcs.CommentData{
			ExternalID:  extID,
			CommentType: "review",
			Author:      rv.User.Login,
			Body:        rv.Body,
			IsResolved:  resolved,
			CreatedAt:   rv.SubmittedAt,
			UpdatedAt:   rv.SubmittedAt,
		})
	}
	return result, nil
}

// apiPaginated calls `gh api <endpoint>` and returns the raw JSON bytes.
func (r *Runner) apiPaginated(ctx context.Context, endpoint string) ([]byte, error) {
	return r.run(ctx, []string{"api", "--paginate", endpoint})
}

// run executes gh with the given args and returns stdout.
// When a stored token is set and the call fails with a 404/403 (e.g. org SSO not authorized),
// it retries once without the token so gh falls back to its native keychain auth.
func (r *Runner) run(ctx context.Context, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	r.mu.RLock()
	tok := r.token
	r.mu.RUnlock()

	out, err := r.exec(ctx, args, tok)
	if err != nil && tok != "" && isAccessError(err) {
		// Stored token may lack org SSO authorization — retry with gh's native keychain.
		out, err = r.exec(ctx, args, "")
	}
	return out, err
}

func isAccessError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") ||
		strings.Contains(msg, "403") || strings.Contains(msg, "Forbidden") ||
		strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized")
}

func (r *Runner) exec(ctx context.Context, args []string, tok string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.binPath, args...) //nolint:gosec
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if tok != "" {
		cmd.Env = append(os.Environ(), "GH_TOKEN="+tok)
	}

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		subcmd := ""
		if len(args) > 0 {
			subcmd = args[0]
		}
		return nil, fmt.Errorf("gh %s: %s", subcmd, errMsg)
	}
	return stdout.Bytes(), nil
}

// resolveBinPath looks up the binary path by checking PATH then fallback dirs.
func resolveBinPath(name string, extraDirs []string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range extraDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name
}

// aggregateCIStatus computes a single CI status from GitHub's statusCheckRollup state values.
// Returns "FAILURE" if any check failed, "PENDING" if any are still running,
// "SUCCESS" if all passed, or "" when no checks are configured.
func aggregateCIStatus(states []string) string {
	if len(states) == 0 {
		return ""
	}
	hasFailure, hasPending := false, false
	for _, s := range states {
		switch s {
		case "FAILURE", "ERROR", "STARTUP_FAILURE", "ACTION_REQUIRED":
			hasFailure = true
		case "PENDING", "IN_PROGRESS", "QUEUED", "WAITING", "REQUESTED":
			hasPending = true
		}
	}
	if hasFailure {
		return "FAILURE"
	}
	if hasPending {
		return "PENDING"
	}
	return "SUCCESS"
}

// mapReviewDecision converts a GitHub reviewDecision enum to our internal review_status.
func mapReviewDecision(d string) string {
	switch d {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes_requested"
	case "REVIEW_REQUIRED":
		return "review_required"
	case "":
		return "review_required"
	default:
		return strings.ToLower(d)
	}
}
