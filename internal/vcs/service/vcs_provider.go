// Package vcsservice implements the VCS provider for the context consolidator.
package vcsservice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/artschekoff/context-consolidator/internal/provider"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/artschekoff/context-consolidator/internal/vcs"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"go.uber.org/zap"
)

// parseVCSTime tries common ISO 8601 layouts and returns zero time on failure.
func parseVCSTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// formatVCSDateTime formats a stored date string as "YYYY-MM-DD HH:MM".
func formatVCSDateTime(s string) string {
	t := parseVCSTime(s)
	if t.IsZero() {
		return s
	}
	return t.Format("2006-01-02 15:04")
}

// NotifyFunc is called when a tracked PR undergoes a notable state change.
// contextID identifies the context; entityID is the internal vcs_pull_requests.id.
// eventType is one of: "pr_conflict", "pr_review_changed".
type NotifyFunc func(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error

// VCSProvider implements provider.Provider for VCS platforms (GitHub, GitLab, Linear).
// It dispatches to the appropriate VCSRunner based on the platform stored with each repo.
type VCSProvider struct {
	store     *vcsstore.Store
	runners   map[vcs.Platform]vcs.VCSRunner
	logger    *zap.Logger
	notify    NotifyFunc // nil = no-op
	msgWriter appstore.MessageWriter
}

// NewVCSProvider constructs a VCSProvider with the given runner implementations.
// Only runners for enabled platforms should be passed.
func NewVCSProvider(store *vcsstore.Store, logger *zap.Logger, msgWriter appstore.MessageWriter, runners ...vcs.VCSRunner) *VCSProvider {
	rm := make(map[vcs.Platform]vcs.VCSRunner, len(runners))
	for _, r := range runners {
		rm[r.Platform()] = r
	}
	return &VCSProvider{store: store, runners: rm, logger: logger, msgWriter: msgWriter}
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

	if len(branches) == 0 {
		return provider.Stats{}, nil
	}

	var (
		mu    sync.Mutex
		stats provider.Stats
		wg    sync.WaitGroup
	)

	workers := min(4, len(branches))
	jobs := make(chan vcsstore.ContextVCSBranch, len(branches))
	for _, b := range branches {
		jobs <- b
	}
	close(jobs)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for branch := range jobs {
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
						pr.Title, pr.Author, pr.State, pr.ReviewStatus, pr.URL, pr.HeadBranch,
						pr.Mergeable, pr.CIStatus, pr.IsDraft,
						pr.CreatedAt, pr.UpdatedAt,
					)
					if err != nil {
						p.logger.Warn("vcs sync: upsert PR failed",
							zap.Int("pr", pr.Number), zap.Error(err))
						continue
					}
					mu.Lock()
					stats.ItemsRefreshed++
					mu.Unlock()

					var prMsgID string
					if p.msgWriter != nil {
						prExtID := branch.Repo.Owner + "/" + branch.Repo.Name + "#" + fmt.Sprint(pr.Number)
						prAuthorID := p.msgWriter.ResolveAuthor(ctx, "github", pr.Author)
						prMsgID, _ = p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
							ContextID:  contextID,
							SourceID:   "github",
							ExternalID: prExtID,
							Title:      pr.Title,
							RawAuthor:  pr.Author,
							AuthorID:   prAuthorID,
							CreatedAt:  pr.CreatedAt,
							UpdatedAt:  pr.UpdatedAt,
						})
						if prMsgID != "" {
							_ = p.store.PullRequests.SetPRMessageID(ctx, res.ID, prMsgID)
						}
					}

					if p.notify != nil && !res.IsNew {
						// PR was merged or closed.
						if res.StateChanged && (pr.State == "merged" || pr.State == "closed") {
							eventType := "pr_" + pr.State // pr_merged | pr_closed
							if notifyErr := p.notify(ctx, contextID, "pull_request", res.ID,
								eventType, res.OldState, pr.State); notifyErr != nil {
								p.logger.Warn("vcs sync: create state notification failed", zap.Error(notifyErr))
							} else {
								mu.Lock()
								stats.NewNotifications++
								mu.Unlock()
							}
						}
						// PR newly became un-mergeable.
						if res.MergeableChanged && pr.Mergeable == "CONFLICTING" {
							if notifyErr := p.notify(ctx, contextID, "pull_request", res.ID,
								"pr_conflict", res.OldMergeable, pr.Mergeable); notifyErr != nil {
								p.logger.Warn("vcs sync: create conflict notification failed", zap.Error(notifyErr))
							} else {
								mu.Lock()
								stats.NewNotifications++
								mu.Unlock()
							}
						}
						// Review status changed (approved, changes_requested, …).
						if res.ReviewStatusChanged && pr.ReviewStatus != "" {
							// If everything is now green, use ready_to_merge instead.
							eventType := "pr_review_changed"
							if pr.ReviewStatus == "approved" &&
								pr.Mergeable == "MERGEABLE" &&
								(pr.CIStatus == "SUCCESS" || pr.CIStatus == "") {
								eventType = "ready_to_merge"
							}
							if notifyErr := p.notify(ctx, contextID, "pull_request", res.ID,
								eventType, res.OldReviewStatus, pr.ReviewStatus); notifyErr != nil {
								p.logger.Warn("vcs sync: create review notification failed", zap.Error(notifyErr))
							} else {
								mu.Lock()
								stats.NewNotifications++
								mu.Unlock()
							}
						}
						// CI status changed.
						if res.CIStatusChanged && pr.CIStatus != "" {
							eventType := "ci_" + strings.ToLower(pr.CIStatus) // ci_success, ci_failure, ci_pending
							// Upgrade to ready_to_merge if all conditions are now met.
							if pr.CIStatus == "SUCCESS" &&
								pr.ReviewStatus == "approved" &&
								pr.Mergeable == "MERGEABLE" {
								eventType = "ready_to_merge"
							}
							if notifyErr := p.notify(ctx, contextID, "pull_request", res.ID,
								eventType, res.OldCIStatus, pr.CIStatus); notifyErr != nil {
								p.logger.Warn("vcs sync: create CI notification failed", zap.Error(notifyErr))
							} else {
								mu.Lock()
								stats.NewNotifications++
								mu.Unlock()
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
						cID, err := p.store.PullRequests.UpsertComment(ctx,
							prID, c.ExternalID, nil, c.CommentType, c.Author, c.Body,
							c.Path, c.Line, c.IsResolved, c.InReplyToID,
							c.CreatedAt, c.UpdatedAt,
						)
						if err != nil {
							p.logger.Warn("vcs sync: upsert comment failed",
								zap.String("externalID", c.ExternalID), zap.Error(err))
							continue
						}
						mu.Lock()
						stats.NewItems++
						mu.Unlock()

						if p.msgWriter != nil && cID != "" && prMsgID != "" {
							cAuthorID := p.msgWriter.ResolveAuthor(ctx, "github", c.Author)
							cMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
								ContextID:  contextID,
								SourceID:   "github",
								ExternalID: c.ExternalID,
								ParentID:   prMsgID,
								Body:       c.Body,
								RawAuthor:  c.Author,
								AuthorID:   cAuthorID,
								CreatedAt:  c.CreatedAt,
								UpdatedAt:  c.UpdatedAt,
							})
							if cMsgID != "" {
								_ = p.store.PullRequests.SetPRCommentMessageID(ctx, cID, cMsgID)
							}
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return stats, nil
}

// CollectItems returns one Item per PR for global sorting.
// Only items on or after startAt are included; pass a zero time for no filter.
func (p *VCSProvider) CollectItems(ctx context.Context, contextID string, startAt time.Time, _ bool) ([]provider.Item, error) {
	prs, err := p.store.PullRequests.ListPRsForContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("vcs collect items: list prs: %w", err)
	}

	var items []provider.Item
	for _, pr := range prs {
		ts := parseVCSTime(pr.PRUpdatedAt)
		if ts.IsZero() {
			ts = parseVCSTime(pr.PRCreatedAt)
		}
		if !startAt.IsZero() && !ts.IsZero() && ts.Before(startAt) {
			continue
		}

		comments, err := p.store.PullRequests.ListCommentsForPRFiltered(ctx, pr.ID, false)
		if err != nil {
			p.logger.Warn("vcs collect items: list comments failed", zap.String("prID", pr.ID), zap.Error(err))
		}

		var buf strings.Builder
		fmt.Fprintf(&buf, "[%s · PR] [PR #%d] %s\n", formatVCSDateTime(pr.PRUpdatedAt), pr.PRNumber, pr.Title)
		fmt.Fprintf(&buf, "Author: %s | State: %s | Review: %s\n", pr.Author, pr.State, pr.ReviewStatus)
		if pr.URL != "" {
			fmt.Fprintf(&buf, "URL: %s\n", pr.URL)
		}
		tree := vcsstore.BuildCommentTree(comments)
		if len(tree) > 0 {
			fmt.Fprintf(&buf, "\nUnresolved comments (%d):\n", len(tree))
			for _, c := range tree {
				writeComment(&buf, c, "  ")
			}
		}
		fmt.Fprint(&buf, "\n")
		items = append(items, provider.Item{CreatedAt: ts, Body: buf.String()})
	}
	return items, nil
}

func writeComment(w *strings.Builder, c vcsstore.PRComment, indent string) {
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
	fmt.Fprintf(w, "%s%s%s:\n", indent, author, location)
	for _, line := range strings.Split(c.Body, "\n") {
		fmt.Fprintf(w, "%s  %s\n", indent, line)
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
		requested, err := runner.ListReviewRequests(ctx, repo.Owner, repo.Name)
		if err != nil {
			p.logger.Warn("vcs: list review requests failed",
				zap.String("repo", repo.FullName()), zap.Error(err))
			continue
		}

		changesRequested, err := runner.ListChangesRequestedByUser(ctx, repo.Owner, repo.Name)
		if err != nil {
			// Non-fatal: log and proceed with whatever we have from the first query.
			p.logger.Warn("vcs: list changes-requested PRs failed",
				zap.String("repo", repo.FullName()), zap.Error(err))
		}

		// Merge both result sets, deduplicating by PR number. PRs appearing in both
		// lists (e.g. still in review-requested AND already reviewed as changes_requested)
		// are kept once; the first-seen review_status wins.
		merged := make(map[int]vcs.PRData, len(requested)+len(changesRequested))
		for _, pr := range requested {
			merged[pr.Number] = pr
		}
		for _, pr := range changesRequested {
			if _, exists := merged[pr.Number]; !exists {
				merged[pr.Number] = pr
			}
		}

		// Upsert each active PR (clears reviewed_at/dismissed_at if it re-appears).
		activeNumbers := make([]int, 0, len(merged))
		for _, pr := range merged {
			activeNumbers = append(activeNumbers, pr.Number)
			if err := p.store.ReviewRequests.UpsertReviewRequest(ctx,
				repo.ID, pr.Number, pr.Title, pr.Author, pr.URL,
				pr.ReviewStatus, pr.HeadBranch, pr.CreatedAt,
			); err != nil {
				p.logger.Warn("vcs: upsert review request failed",
					zap.Int("pr", pr.Number), zap.Error(err))
			}
		}

		// Mark records that disappeared from both GitHub lists as "reviewed"
		// (author needs to act). This preserves them in the UI under a
		// "Waiting for Author" section instead of deleting.
		if err := p.store.ReviewRequests.MarkReviewedForRepo(ctx, repo.ID, activeNumbers); err != nil {
			p.logger.Warn("vcs: mark reviewed failed", zap.String("repo", repo.FullName()), zap.Error(err))
		}

		// Relabel dropped PRs that were actually merged or closed, so the UI shows
		// "Merged"/"Closed" instead of "Waiting for Author".
		// ponytail: one `gh pr view` per newly-dropped PR; PRs already labeled
		// terminal are skipped, so each PR is fetched at most once.
		reconcileTerminalStates(ctx, p, runner, repo, activeNumbers)
	}
	return nil
}

// reconcileTerminalStates fetches the real lifecycle state of review requests that
// dropped off the open review lists and records "merged"/"closed" in review_status.
// Best-effort: any failure leaves a PR labeled as-is.
func reconcileTerminalStates(ctx context.Context, p *VCSProvider, runner vcs.VCSRunner, repo vcsstore.VCSRepo, activeNumbers []int) {
	stored, err := p.store.ReviewRequests.ListReviewRequests(ctx, repo.ID)
	if err != nil {
		p.logger.Warn("vcs: list stored review requests failed",
			zap.String("repo", repo.FullName()), zap.Error(err))
		return
	}
	active := make(map[int]struct{}, len(activeNumbers))
	for _, n := range activeNumbers {
		active[n] = struct{}{}
	}
	for _, rr := range stored {
		if _, isActive := active[rr.PRNumber]; isActive {
			continue // still in an open review list
		}
		if rr.ReviewStatus == "merged" || rr.ReviewStatus == "closed" {
			continue // already terminal — do not re-fetch
		}
		pr, err := runner.GetPRByNumber(ctx, repo.Owner, repo.Name, rr.PRNumber)
		if err != nil || pr == nil {
			continue // best-effort
		}
		if pr.State == "merged" || pr.State == "closed" {
			if err := p.store.ReviewRequests.SetReviewStatus(ctx, repo.ID, rr.PRNumber, pr.State); err != nil {
				p.logger.Warn("vcs: set review status failed",
					zap.Int("pr", rr.PRNumber), zap.Error(err))
			}
		}
	}
}

// VCSAuth stores the GitHub auth result. Delegates to the auth store.
func (p *VCSProvider) SaveAuth(ctx context.Context, platform vcs.Platform, username, avatarURL, token string) error {
	_, err := p.store.Auth.UpsertAuth(ctx, string(platform), username, avatarURL, token)
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
	runner.SetToken(result.Token)
	if err := p.SaveAuth(ctx, platform, result.Username, result.AvatarURL, result.Token); err != nil {
		p.logger.Warn("vcs: save auth failed", zap.Error(err))
	}
	return result, nil
}
