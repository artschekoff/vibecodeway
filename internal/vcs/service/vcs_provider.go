// Package vcsservice implements the VCS provider for the context consolidator.
package vcsservice

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artschekoff/context-consolidator/internal/provider"
	"github.com/artschekoff/context-consolidator/internal/vcs"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"go.uber.org/zap"
)

// NotifyFunc is called when a tracked PR undergoes a notable state change.
// contextID identifies the context; entityID is the internal vcs_pull_requests.id.
// eventType is one of: "pr_conflict", "pr_review_changed".
type NotifyFunc func(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error

// VCSProvider implements provider.Provider for VCS platforms (GitHub, GitLab, Linear).
// It dispatches to the appropriate VCSRunner based on the platform stored with each repo.
type VCSProvider struct {
	store   *vcsstore.Store
	runners map[vcs.Platform]vcs.VCSRunner
	logger  *zap.Logger
	notify  NotifyFunc // nil = no-op
}

// NewVCSProvider constructs a VCSProvider with the given runner implementations.
// Only runners for enabled platforms should be passed.
func NewVCSProvider(store *vcsstore.Store, logger *zap.Logger, runners ...vcs.VCSRunner) *VCSProvider {
	rm := make(map[vcs.Platform]vcs.VCSRunner, len(runners))
	for _, r := range runners {
		rm[r.Platform()] = r
	}
	return &VCSProvider{store: store, runners: rm, logger: logger}
}

// WithNotifyFunc sets the callback that is invoked when a notable PR state change is detected.
func (p *VCSProvider) WithNotifyFunc(fn NotifyFunc) { p.notify = fn }

// Name implements provider.Provider.
func (p *VCSProvider) Name() string { return "vcs" }

// IsConfigured implements provider.Provider.
func (p *VCSProvider) IsConfigured(ctx context.Context, contextID string) (bool, error) {
	branches, err := p.store.Branches.ListBranches(ctx, contextID)
	if err != nil {
		return false, err
	}
	return len(branches) > 0, nil
}

// Refresh re-fetches PRs and comments for all branches already stored for contextID.
// Stats.ItemsRefreshed = PRs upserted; Stats.NewItems = comments upserted.
func (p *VCSProvider) Refresh(ctx context.Context, contextID string) (provider.Stats, error) {
	return p.sync(ctx, contextID)
}

// Reload performs the same operation as Refresh (branches are user-managed, not discovered).
func (p *VCSProvider) Reload(ctx context.Context, contextID string) (provider.Stats, error) {
	return p.sync(ctx, contextID)
}

// sync is the shared implementation for Refresh and Reload.
func (p *VCSProvider) sync(ctx context.Context, contextID string) (provider.Stats, error) {
	branches, err := p.store.Branches.ListBranches(ctx, contextID)
	if err != nil {
		return provider.Stats{}, fmt.Errorf("vcs sync: list branches: %w", err)
	}

	stats := provider.Stats{}
	for _, branch := range branches {
		if branch.Repo == nil {
			continue
		}
		platform := vcs.Platform(branch.Repo.Platform)
		runner, ok := p.runners[platform]
		if !ok {
			p.logger.Warn("vcs sync: no runner for platform",
				zap.String("platform", string(platform)),
				zap.String("repo", branch.Repo.FullName()),
			)
			continue
		}

		prs, err := runner.ListPRsForBranch(ctx, branch.Repo.Owner, branch.Repo.Name, branch.Branch)
		if err != nil {
			p.logger.Warn("vcs sync: list PRs failed",
				zap.String("repo", branch.Repo.FullName()),
				zap.String("branch", branch.Branch),
				zap.Error(err),
			)
			continue
		}

		for _, pr := range prs {
			res, err := p.store.PullRequests.UpsertPR(ctx,
				branch.RepoID, branch.ID, pr.Number,
				pr.Title, pr.Author, pr.State, pr.ReviewStatus, pr.URL, pr.HeadBranch, pr.Mergeable,
				pr.CreatedAt, pr.UpdatedAt,
			)
			if err != nil {
				p.logger.Warn("vcs sync: upsert PR failed",
					zap.Int("pr", pr.Number), zap.Error(err))
				continue
			}
			stats.ItemsRefreshed++

			if p.notify != nil && !res.IsNew {
				// Fire when the PR newly became un-mergeable.
				if res.MergeableChanged && pr.Mergeable == "CONFLICTING" {
					if err := p.notify(ctx, contextID, "pull_request", res.ID,
						"pr_conflict", res.OldMergeable, pr.Mergeable); err != nil {
						p.logger.Warn("vcs sync: create conflict notification failed", zap.Error(err))
					} else {
						stats.NewNotifications++
					}
				}
				// Fire when review status changed (e.g. approved, changes_requested).
				if res.ReviewStatusChanged && pr.ReviewStatus != "" {
					if err := p.notify(ctx, contextID, "pull_request", res.ID,
						"pr_review_changed", res.OldReviewStatus, pr.ReviewStatus); err != nil {
						p.logger.Warn("vcs sync: create review notification failed", zap.Error(err))
					} else {
						stats.NewNotifications++
					}
				}
			}

			prID := res.ID

			comments, err := runner.GetPRComments(ctx, branch.Repo.Owner, branch.Repo.Name, pr.Number)
			if err != nil {
				p.logger.Warn("vcs sync: get comments failed",
					zap.Int("pr", pr.Number), zap.Error(err))
				continue
			}

			for _, c := range comments {
				if err := p.store.PullRequests.UpsertComment(ctx,
					prID, c.ExternalID, nil, c.CommentType, c.Author, c.Body,
					c.Path, c.Line, c.IsResolved, c.InReplyToID,
					c.CreatedAt, c.UpdatedAt,
				); err != nil {
					p.logger.Warn("vcs sync: upsert comment failed",
						zap.String("externalID", c.ExternalID), zap.Error(err))
					continue
				}
				stats.NewItems++
			}
		}
	}
	return stats, nil
}

// WriteContext appends a human-readable PR section for contextID to w.
// Only items on or after startAt are included; pass a zero time for no filter.
func (p *VCSProvider) WriteContext(ctx context.Context, w io.Writer, contextID string, startAt time.Time) error {
	prs, err := p.store.PullRequests.ListPRsForContext(ctx, contextID)
	if err != nil {
		return fmt.Errorf("vcs write context: list prs: %w", err)
	}
	if len(prs) == 0 {
		return nil
	}

	_, _ = fmt.Fprintf(w, "--- PULL REQUESTS (%d) ---\n\n", len(prs))

	for _, pr := range prs {
		// Apply startAt filter by PR updated time.
		if !startAt.IsZero() {
			t, _ := time.Parse(time.RFC3339, pr.PRUpdatedAt)
			if !t.IsZero() && t.Before(startAt) {
				continue
			}
		}

		_, _ = fmt.Fprintf(w, "[PR #%d] %s\n", pr.PRNumber, pr.Title)
		_, _ = fmt.Fprintf(w, "Author: %s | State: %s | Review: %s\n", pr.Author, pr.State, pr.ReviewStatus)
		if pr.URL != "" {
			_, _ = fmt.Fprintf(w, "URL: %s\n", pr.URL)
		}

		// Load and print unresolved comments.
		comments, err := p.store.PullRequests.ListCommentsForPRFiltered(ctx, pr.ID, false)
		if err != nil {
			p.logger.Warn("vcs write context: list comments failed", zap.String("prID", pr.ID), zap.Error(err))
			_, _ = fmt.Fprint(w, "\n")
			continue
		}

		tree := vcsstore.BuildCommentTree(comments)
		if len(tree) > 0 {
			_, _ = fmt.Fprintf(w, "\nUnresolved comments (%d):\n", len(tree))
			for _, c := range tree {
				writeComment(w, c, "  ")
			}
		}
		_, _ = fmt.Fprint(w, "\n")
	}
	return nil
}

func writeComment(w io.Writer, c vcsstore.PRComment, indent string) {
	author := c.Author
	if author == "" {
		author = "Unknown"
	}
	location := ""
	if c.Path != nil && *c.Path != "" {
		location = fmt.Sprintf(" [%s", *c.Path)
		if c.Line != nil {
			location += fmt.Sprintf(":%d", *c.Line)
		}
		location += "]"
	}
	_, _ = fmt.Fprintf(w, "%s%s%s:\n", indent, author, location)
	for _, line := range strings.Split(c.Body, "\n") {
		_, _ = fmt.Fprintf(w, "%s  %s\n", indent, line)
	}
	for _, reply := range c.Replies {
		writeComment(w, reply, indent+"  ")
	}
}

// SyncReviewRequests fetches review-requested PRs for the authenticated user across all
// tracked repos for a given platform and stores them in the DB.
func (p *VCSProvider) SyncReviewRequests(ctx context.Context, platform vcs.Platform) error {
	runner, ok := p.runners[platform]
	if !ok {
		return fmt.Errorf("no runner for platform %q", platform)
	}

	repos, err := p.store.Repos.ListRepos(ctx, string(platform))
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}

	for _, repo := range repos {
		prs, err := runner.ListReviewRequests(ctx, repo.Owner, repo.Name)
		if err != nil {
			p.logger.Warn("vcs: list review requests failed",
				zap.String("repo", repo.FullName()), zap.Error(err))
			continue
		}

		// Delete stale rows for this repo before upserting fresh ones.
		if err := p.store.ReviewRequests.DeleteReviewRequestsForRepo(ctx, repo.ID); err != nil {
			p.logger.Warn("vcs: delete review requests failed", zap.String("repo", repo.FullName()), zap.Error(err))
		}

		for _, pr := range prs {
			if err := p.store.ReviewRequests.UpsertReviewRequest(ctx,
				repo.ID, pr.Number, pr.Title, pr.Author, pr.URL,
				pr.ReviewStatus, pr.CreatedAt,
			); err != nil {
				p.logger.Warn("vcs: upsert review request failed",
					zap.Int("pr", pr.Number), zap.Error(err))
			}
		}
	}
	return nil
}

// VCSAuth stores the GitHub auth result. Delegates to the auth store.
func (p *VCSProvider) SaveAuth(ctx context.Context, platform vcs.Platform, username, avatarURL string) error {
	_, err := p.store.Auth.UpsertAuth(ctx, string(platform), username, avatarURL)
	return err
}

// GetAuthUser calls the runner to verify current auth status.
func (p *VCSProvider) GetAuthUser(ctx context.Context, platform vcs.Platform) (string, error) {
	runner, ok := p.runners[platform]
	if !ok {
		return "", fmt.Errorf("no runner for platform %q", platform)
	}
	return runner.GetAuthUser(ctx)
}

// LoginStart initiates the device-code flow and returns the code info for the UI to display.
func (p *VCSProvider) LoginStart(ctx context.Context, platform vcs.Platform) (*vcs.DeviceCodeInfo, error) {
	runner, ok := p.runners[platform]
	if !ok {
		return nil, fmt.Errorf("platform %q is not supported yet", platform)
	}
	return runner.StartDeviceAuth(ctx)
}

// RunnerFor returns the VCSRunner for the given platform, or an error if not found.
func (p *VCSProvider) RunnerFor(platform vcs.Platform) (vcs.VCSRunner, error) {
	runner, ok := p.runners[platform]
	if !ok {
		return nil, fmt.Errorf("no runner for platform %q", platform)
	}
	return runner, nil
}

// LoginComplete polls until the device-code flow finishes, persists auth, and returns the result.
func (p *VCSProvider) LoginComplete(ctx context.Context, platform vcs.Platform, deviceCode string, interval int) (vcs.AuthResult, error) {
	runner, ok := p.runners[platform]
	if !ok {
		return vcs.AuthResult{}, fmt.Errorf("platform %q is not supported yet", platform)
	}
	result, err := runner.CompleteDeviceAuth(ctx, deviceCode, interval)
	if err != nil {
		return vcs.AuthResult{}, err
	}
	if err := p.SaveAuth(ctx, platform, result.Username, result.AvatarURL); err != nil {
		p.logger.Warn("vcs: save auth failed", zap.Error(err))
	}
	return result, nil
}
