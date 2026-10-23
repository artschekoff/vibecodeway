// Package vcsservice implements the VCS provider for the context consolidator.
package vcsservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/artschekoff/vibecodeway/internal/provider"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
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

// ReviewNotifyFunc is invoked once when a brand-new review request is inserted.
// repoName, prTitle, and prAuthor are included so callers can record a
// human-readable domain event without a second lookup.
type ReviewNotifyFunc func(ctx context.Context, repoID, repoName string, prNumber int, prTitle, prAuthor string)

// CommentNotifyFunc is invoked once when a brand-new PR comment is inserted.
type CommentNotifyFunc func(ctx context.Context, contextID, prID, externalID, author, body string)

// RecordStatFunc, when set, persists an analytics stat event. Best-effort:
// callers must log and continue on error, never fail the sync.
type RecordStatFunc func(ctx context.Context, kind, externalID, org, repo, jiraKey, occurredAt string)

// prCacheTTL bounds how long a repo's fetched-all PR list is reused across branch
// worker iterations before a fresh `gh pr list` call is made for that repo.
const prCacheTTL = 5 * time.Minute

// prCacheEntry is one repo's cached PR list, fetched via ListPRsForBranch with
// branch == "" (fetch-all mode) and reused for every branch tracked in that repo.
type prCacheEntry struct {
	prs       []vcs.PRData
	fetchedAt time.Time
	// truncated is true when prs hit vcs.FetchAllPRLimit, meaning older PRs may have
	// been cut off the batch — a tracked branch with no match in prs is ambiguous
	// ("no PR" vs "PR fell off the page") until a scoped per-branch query rules it out.
	truncated bool
}

// VCSProvider implements provider.Provider for VCS platforms (GitHub, GitLab, Linear).
// It dispatches to the appropriate VCSRunner based on the platform stored with each repo.
type VCSProvider struct {
	store         *vcsstore.Store
	runners       map[vcs.Platform]vcs.VCSRunner
	logger        *zap.Logger
	notify        NotifyFunc        // nil = no-op
	reviewNotify  ReviewNotifyFunc  // nil = no-op
	commentNotify CommentNotifyFunc // nil = no-op
	recordStat    RecordStatFunc    // nil = no-op
	// reviewProgress reports per-repo progress of a reviews refresh so the UI can
	// draw a live progress bar. phase is "sync" (combined), "requested" or "authored". nil = no-op.
	reviewProgress func(current, total int, repo, phase string)
	msgWriter      appstore.MessageWriter

	// prCache holds one fetch-all PR list per repo (keyed "owner/name"), shared by
	// every branch tracked in that repo during sync(). Guarded by prCacheMu since
	// the branch worker pool calls listRepoPRsCached concurrently.
	prCacheMu sync.Mutex
	prCache   map[string]prCacheEntry
	// reviewSyncMu serialises SyncReviews (see ErrReviewSyncRunning).
	reviewSyncMu sync.Mutex
}

// NewVCSProvider constructs a VCSProvider with the given runner implementations.
// Only runners for enabled platforms should be passed.
func NewVCSProvider(store *vcsstore.Store, logger *zap.Logger, msgWriter appstore.MessageWriter, runners ...vcs.VCSRunner) *VCSProvider {
	rm := make(map[vcs.Platform]vcs.VCSRunner, len(runners))
	for _, r := range runners {
		rm[r.Platform()] = r
	}
	return &VCSProvider{store: store, runners: rm, logger: logger, msgWriter: msgWriter, prCache: make(map[string]prCacheEntry)}
}

// listRepoPRsCached returns all PRs for owner/name (fetch-all mode), reusing a
// cached result when one was fetched within prCacheTTL. This turns "one gh pr list
// call per branch" into "one call per repo per TTL window" — with ~39 contexts
// often sharing a handful of repos across many tracked branches, this is the
// difference between minutes and tens of minutes for a full context refresh.
//
// ponytail: no per-key in-flight guard. Two branches in the same repo processed
// concurrently by different pool workers can both miss the cache and each fire
// their own gh pr list call before either writes back — a bounded duplicate (pool
// caps concurrency at 4), not unbounded. A guard that serializes on the key would
// require holding a lock across the network call for that key, which is fine, but
// serializing the fetch across *all* repos (a single provider-wide lock) is not
// acceptable — that would re-introduce the sequential-fetch bottleneck this cache
// exists to remove. Rare duplicate fetches are an acceptable ceiling here.
// listRepoPRsCached returns (prs, truncated). truncated is true when the batch result
// hit vcs.FetchAllPRLimit — see prCacheEntry.truncated.
func (p *VCSProvider) listRepoPRsCached(ctx context.Context, runner vcs.VCSRunner, owner, name string) ([]vcs.PRData, bool, error) {
	key := owner + "/" + name

	p.prCacheMu.Lock()
	entry, ok := p.prCache[key]
	p.prCacheMu.Unlock()
	if ok && time.Since(entry.fetchedAt) < prCacheTTL {
		return entry.prs, entry.truncated, nil
	}

	prs, err := runner.ListPRsForBranch(ctx, owner, name, "")
	if err != nil {
		// The fetch-all batch (--state all + statusCheckRollup, up to 200 PRs)
		// is heavy enough that GitHub 504s on busy repos. Treat a failed batch
		// like a fully truncated one: cache the failure for the TTL and return
		// truncated=true, so every tracked branch in this repo resolves through
		// the cheap scoped --head fallback instead of re-hitting the timeout.
		p.logger.Warn("vcs sync: fetch-all PR batch failed; per-branch fallback will be used",
			zap.String("repo", key), zap.Error(err))
		p.prCacheMu.Lock()
		p.prCache[key] = prCacheEntry{prs: nil, fetchedAt: time.Now(), truncated: true}
		p.prCacheMu.Unlock()
		return nil, true, nil
	}
	truncated := len(prs) >= vcs.FetchAllPRLimit

	p.prCacheMu.Lock()
	p.prCache[key] = prCacheEntry{prs: prs, fetchedAt: time.Now(), truncated: truncated}
	p.prCacheMu.Unlock()

	return prs, truncated, nil
}

// InvalidatePRCache clears the repo-PR cache used by sync(), forcing the next
// sync to re-fetch every repo instead of reusing a cached entry. Called by
// App.RefreshContext (the user-triggered manual refresh) before it delegates —
// the periodic background sync (RefreshAllContexts) does not call this and keeps
// relying on the TTL, which is the point of the cache.
//
// ponytail: TTL tradeoff — a PR opened seconds ago may not appear in a context's
// *background-synced* PR list until prCacheTTL elapses or a manual refresh clears
// this cache. Acceptable because GetPRByNumber (used when a user attaches a
// specific PR URL) bypasses the cache entirely and always fetches live, and the
// manual refresh path above now also always sees fresh data.
func (p *VCSProvider) InvalidatePRCache() {
	p.prCacheMu.Lock()
	p.prCache = make(map[string]prCacheEntry)
	p.prCacheMu.Unlock()
}

// WithNotifyFunc sets the callback that is invoked when a notable PR state change is detected.
func (p *VCSProvider) WithNotifyFunc(fn NotifyFunc) { p.notify = fn }

// WithReviewNotifyFunc sets the callback invoked when a new review request appears.
func (p *VCSProvider) WithReviewNotifyFunc(fn ReviewNotifyFunc) { p.reviewNotify = fn }

// WithCommentNotifyFunc sets the callback invoked when a new PR comment appears.
func (p *VCSProvider) WithCommentNotifyFunc(fn CommentNotifyFunc) { p.commentNotify = fn }

// WithRecordStatFunc sets the callback used to persist analytics stat events.
func (p *VCSProvider) WithRecordStatFunc(fn RecordStatFunc) { p.recordStat = fn }

// WithReviewProgressFunc sets the callback that reports reviews-refresh progress.
func (p *VCSProvider) WithReviewProgressFunc(fn func(current, total int, repo, phase string)) {
	p.reviewProgress = fn
}

// emitReviewProgress is a nil-safe helper for the reviews-refresh progress callback.
func (p *VCSProvider) emitReviewProgress(current, total int, repo, phase string) {
	if p.reviewProgress != nil {
		p.reviewProgress(current, total, repo, phase)
	}
}

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
	return p.sync(ctx, contextID, nil)
}

// RefreshWithProgress implements provider.ProgressReporter: report gets the
// repo/branch currently being synced.
func (p *VCSProvider) RefreshWithProgress(ctx context.Context, contextID string, report provider.ProgressFunc) (provider.Stats, error) {
	return p.sync(ctx, contextID, report)
}

// Reload performs the same operation as Refresh (branches are user-managed, not discovered).
func (p *VCSProvider) Reload(ctx context.Context, contextID string) (provider.Stats, error) {
	return p.sync(ctx, contextID, nil)
}

// sync is the shared implementation for Refresh and Reload.
func (p *VCSProvider) sync(ctx context.Context, contextID string, report provider.ProgressFunc) (provider.Stats, error) {
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
				// "Autodetect PR" off: the user wants the branch tracked (e.g. main)
				// without dragging in its pull requests.
				if !branch.AutodetectPR {
					continue
				}
				if report != nil {
					report(branch.Repo.Name + " " + branch.Branch)
				}
				platform := vcs.Platform(branch.Repo.Platform)
				runner, ok := p.runners[platform]
				if !ok {
					p.logger.Warn(
						"vcs sync: no runner for platform",
						zap.String("platform", string(platform)),
						zap.String("repo", branch.Repo.FullName()),
					)
					continue
				}

				allPRs, truncated, err := p.listRepoPRsCached(ctx, runner, branch.Repo.Owner, branch.Repo.Name)
				if err != nil {
					p.logger.Warn(
						"vcs sync: list PRs failed",
						zap.String("repo", branch.Repo.FullName()),
						zap.String("branch", branch.Branch),
						zap.Error(err),
					)
					continue
				}

				var prs []vcs.PRData
				for _, pr := range allPRs {
					if pr.HeadBranch == branch.Branch {
						prs = append(prs, pr)
					}
				}

				// The cached batch is capped at vcs.FetchAllPRLimit and ordered
				// newest-first, so a branch with no match in a batch that hit the cap
				// is ambiguous: it may genuinely have no PR, or its PR may be old
				// enough to have fallen off the page. Only in that specific case, pay
				// for one scoped --head query (the pre-batching behavior) to resolve
				// it — a branch with a match, or a batch under the cap, never pays this.
				if len(prs) == 0 && truncated {
					fallbackPRs, fbErr := runner.ListPRsForBranch(ctx, branch.Repo.Owner, branch.Repo.Name, branch.Branch)
					if fbErr != nil {
						p.logger.Warn(
							"vcs sync: per-branch PR fallback failed",
							zap.String("repo", branch.Repo.FullName()),
							zap.String("branch", branch.Branch),
							zap.Error(fbErr),
						)
					} else {
						prs = fallbackPRs
					}
				}

				for _, pr := range prs {
					res, err := p.store.PullRequests.UpsertPR(
						ctx,
						branch.RepoID, branch.ID, pr.Number,
						pr.Title, pr.Author, pr.State, pr.ReviewStatus, pr.URL, pr.HeadBranch,
						pr.Mergeable, pr.MergeStateStatus, pr.CIStatus, pr.IsDraft,
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
						cID, isNew, err := p.store.PullRequests.UpsertComment(
							ctx,
							prID, c.ExternalID, nil, c.CommentType, c.Author, c.Body,
							c.Path, c.Line, c.IsResolved, c.InReplyToID,
							c.CreatedAt, c.UpdatedAt,
						)
						if err != nil {
							p.logger.Warn("vcs sync: upsert comment failed",
								zap.String("externalID", c.ExternalID), zap.Error(err))
							continue
						}
						if isNew && p.commentNotify != nil {
							p.commentNotify(ctx, contextID, prID, cID, c.Author, c.Body)
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

// ErrReviewSyncRunning is returned by SyncReviews when another sync is in
// flight (background ticker vs. manual refresh); the caller just skips.
var ErrReviewSyncRunning = errors.New("reviews sync already running")

// SyncReviews is THE reviews refresh: one pass over the tracked repos doing
// both the review-requested and the authored sync per repo, with a single
// terminal-state reconcile each. Concurrent calls are rejected with
// ErrReviewSyncRunning so two refreshes never interleave.
func (p *VCSProvider) SyncReviews(ctx context.Context, platform vcs.Platform) error {
	if !p.reviewSyncMu.TryLock() {
		return ErrReviewSyncRunning
	}
	defer p.reviewSyncMu.Unlock()
	runner, ok := p.runners[platform]
	if !ok {
		return fmt.Errorf("no runner for platform %q", platform)
	}
	repos, err := p.listReposDeduped(ctx, platform)
	if err != nil {
		return err
	}
	myLogin := p.authLogin(ctx, runner)
	for i, repo := range repos {
		p.emitReviewProgress(i+1, len(repos), repo.FullName(), "sync")
		requested := p.syncRepoRequested(ctx, runner, repo, myLogin)
		authored := p.syncRepoAuthored(ctx, runner, repo)
		reconcileTerminalStates(ctx, p, runner, repo, append(requested, authored...), myLogin)
	}
	return nil
}

// SyncReviewRequests fetches review-requested PRs for the authenticated user across all
// tracked repos (requested kind only). SyncReviews is the combined entry point.
func (p *VCSProvider) SyncReviewRequests(ctx context.Context, platform vcs.Platform) error {
	runner, ok := p.runners[platform]
	if !ok {
		return fmt.Errorf("no runner for platform %q", platform)
	}
	repos, err := p.listReposDeduped(ctx, platform)
	if err != nil {
		return err
	}
	myLogin := p.authLogin(ctx, runner)
	for i, repo := range repos {
		p.emitReviewProgress(i+1, len(repos), repo.FullName(), "requested")
		active := p.syncRepoRequested(ctx, runner, repo, myLogin)
		reconcileTerminalStates(ctx, p, runner, repo, active, myLogin)
	}
	return nil
}

// SyncAuthoredPRs fetches the authenticated user's open authored PRs across all
// tracked repos and stores them as kind='authored' review-request rows.
// It does NOT call MarkReviewedForRepo (that is requester-only).
func (p *VCSProvider) SyncAuthoredPRs(ctx context.Context, platform vcs.Platform) error {
	runner, ok := p.runners[platform]
	if !ok {
		return fmt.Errorf("no runner for platform %q", platform)
	}
	repos, err := p.listReposDeduped(ctx, platform)
	if err != nil {
		return err
	}
	for i, repo := range repos {
		p.emitReviewProgress(i+1, len(repos), repo.FullName(), "authored")
		active := p.syncRepoAuthored(ctx, runner, repo)
		reconcileTerminalStates(ctx, p, runner, repo, active, "")
	}
	return nil
}

// listReposDeduped lists tracked repos with case-duplicates collapsed: GitHub
// owner/name are case-insensitive, and a repo re-added as "Org/Repo" next to
// "org/repo" must be walked (and billed against the API) once.
func (p *VCSProvider) listReposDeduped(ctx context.Context, platform vcs.Platform) ([]vcsstore.VCSRepo, error) {
	repos, err := p.store.Repos.ListRepos(ctx, string(platform))
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	return dedupeRepos(repos), nil
}

func dedupeRepos(repos []vcsstore.VCSRepo) []vcsstore.VCSRepo {
	seen := make(map[string]bool, len(repos))
	out := repos[:0:0]
	for _, r := range repos {
		k := strings.ToLower(r.Platform + "/" + r.Owner + "/" + r.Name)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r)
	}
	return out
}

// authLogin resolves the current user's login once per sync. Best-effort — an
// empty login just skips the my-review-state refresh rather than failing the sync.
func (p *VCSProvider) authLogin(ctx context.Context, runner vcs.VCSRunner) string {
	login, err := runner.GetAuthUser(ctx)
	if err != nil {
		p.logger.Warn("vcs: resolve auth user failed", zap.Error(err))
		return ""
	}
	return login
}

// syncRepoRequested upserts the PRs awaiting my review in one repo and marks
// the ones that vanished as reviewed. Returns the active PR numbers.
func (p *VCSProvider) syncRepoRequested(ctx context.Context, runner vcs.VCSRunner, repo vcsstore.VCSRepo, myLogin string) []int {
	requested, err := runner.ListReviewRequests(ctx, repo.Owner, repo.Name)
	if err != nil {
		p.logger.Warn("vcs: list review requests failed",
			zap.String("repo", repo.FullName()), zap.Error(err))
		return nil
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
		if _, exists := merged[pr.Number]; exists {
			continue
		}
		// `reviewed-by:@me review:changes_requested` matches when the PR is in
		// changes-requested state and I reviewed it in ANY way — a mere
		// comment on a PR someone else blocked also hits. So my own PRs are
		// never review work, and the decision counts as mine only when the
		// per-reviewer decisions say so (older runners without reviewer data
		// keep the previous assumption).
		if myLogin != "" && pr.Author == myLogin {
			continue
		}
		mine := pr.Reviewers == nil
		for _, rv := range pr.Reviewers {
			if rv.Login == myLogin && rv.State == "changes_requested" {
				mine = true
			}
		}
		if mine {
			state := "changes_requested"
			pr.MyReviewState = &state
		}
		merged[pr.Number] = pr
	}

	// Upsert each active PR (clears reviewed_at/dismissed_at if it re-appears).
	activeNumbers := make([]int, 0, len(merged))
	for _, pr := range merged {
		activeNumbers = append(activeNumbers, pr.Number)
		reviewersJSON := marshalReviewers(pr.Reviewers)
		insertedRR, err := p.store.ReviewRequests.UpsertReviewRequest(
			ctx,
			repo.ID, pr.Number, pr.Title, pr.Author, pr.URL,
			pr.ReviewStatus, pr.HeadBranch, pr.CreatedAt, reviewersJSON, pr.Mergeable,
			"requested", pr.CIStatus, pr.MergeStateStatus,
		)
		if err != nil {
			p.logger.Warn("vcs: upsert review request failed",
				zap.Int("pr", pr.Number), zap.Error(err))
		}
		if pr.MyReviewState != nil {
			if err := p.store.ReviewRequests.UpdateMyReviewState(ctx, repo.ID, pr.Number, *pr.MyReviewState); err != nil {
				p.logger.Warn("vcs: update my review state failed",
					zap.Int("pr", pr.Number), zap.Error(err))
			}
		}
		if insertedRR && p.reviewNotify != nil {
			p.reviewNotify(ctx, repo.ID, repo.Name, pr.Number, pr.Title, pr.Author)
		}
		p.refreshUnresolvedCount(ctx, runner, repo, pr.Number)
	}

	// Mark records that disappeared from both GitHub lists as "reviewed"
	// (author needs to act). This preserves them in the UI under a
	// "Waiting for Author" section instead of deleting.
	if err := p.store.ReviewRequests.MarkReviewedForRepo(ctx, repo.ID, activeNumbers); err != nil {
		p.logger.Warn("vcs: mark reviewed failed", zap.String("repo", repo.FullName()), zap.Error(err))
	}
	return activeNumbers
}

// syncRepoAuthored upserts my open PRs in one repo as kind='authored'.
// Returns the active PR numbers.
func (p *VCSProvider) syncRepoAuthored(ctx context.Context, runner vcs.VCSRunner, repo vcsstore.VCSRepo) []int {
	authored, err := runner.ListAuthoredPRs(ctx, repo.Owner, repo.Name, "")
	if err != nil {
		p.logger.Warn("vcs: list authored PRs failed",
			zap.String("repo", repo.FullName()), zap.Error(err))
		return nil
	}
	activeNumbers := make([]int, 0, len(authored))
	for _, pr := range authored {
		activeNumbers = append(activeNumbers, pr.Number)
		if _, err := p.store.ReviewRequests.UpsertReviewRequest(
			ctx,
			repo.ID, pr.Number, pr.Title, pr.Author, pr.URL,
			pr.ReviewStatus, pr.HeadBranch, pr.CreatedAt, marshalReviewers(pr.Reviewers), pr.Mergeable,
			"authored", pr.CIStatus, pr.MergeStateStatus,
		); err != nil {
			p.logger.Warn("vcs: upsert authored PR failed", zap.Int("pr", pr.Number), zap.Error(err))
		}
		// My own PR carries no decision of mine (a stale one can linger from a
		// mis-tagged requested row).
		_ = p.store.ReviewRequests.UpdateMyReviewState(ctx, repo.ID, pr.Number, "")
		p.refreshUnresolvedCount(ctx, runner, repo, pr.Number)
	}
	return activeNumbers
}

// refreshUnresolvedCount recomputes the stored unresolved review-thread count for
// one PR. Best-effort: a failed fetch keeps the previous stored value.
func (p *VCSProvider) refreshUnresolvedCount(ctx context.Context, runner vcs.VCSRunner, repo vcsstore.VCSRepo, prNumber int) {
	count, err := runner.CountUnresolvedThreads(ctx, repo.Owner, repo.Name, prNumber)
	if err != nil {
		p.logger.Warn("vcs: count unresolved threads failed", zap.Int("pr", prNumber), zap.Error(err))
		return
	}
	if err := p.store.ReviewRequests.UpdateUnresolvedCount(ctx, repo.ID, prNumber, count); err != nil {
		p.logger.Warn("vcs: update unresolved count failed", zap.Int("pr", prNumber), zap.Error(err))
	}
}

// reconcileTerminalStates fetches the real lifecycle state of review requests that
// dropped off the open review lists and records "merged"/"closed" in review_status.
// Best-effort: any failure leaves a PR labeled as-is.
func reconcileTerminalStates(ctx context.Context, p *VCSProvider, runner vcs.VCSRunner, repo vcsstore.VCSRepo, activeNumbers []int, myLogin string) {
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
			// The user's own PR (kind "authored") transitioning to merged is the
			// merge observation for analytics. PRs the user only reviewed are not
			// theirs to count here.
			if pr.State == "merged" && rr.Kind == "authored" && p.recordStat != nil {
				extID := repo.Owner + "/" + repo.Name + "#" + fmt.Sprint(rr.PRNumber)
				// ponytail: prefer the real merge timestamp gh reports
				// (pr.MergedAt, RFC3339); fall back to observed-at only when
				// it's missing/unparseable so a merge is still recorded.
				occurredAt := time.Now().UTC().Format(time.RFC3339)
				if pr.MergedAt != "" {
					if t, err := time.Parse(time.RFC3339, pr.MergedAt); err == nil {
						occurredAt = t.UTC().Format(time.RFC3339)
					}
				}
				p.recordStat(ctx, "pr_merged", extID, "", repo.FullName(), "", occurredAt)
			}
			continue
		}
		// Still open but dropped from the review-requested list — the user just
		// reviewed it (typically an approval, which itself removes the PR from that
		// search). Refresh the reviewers snapshot so their own decision shows, and
		// the mergeable status so a merge conflict that appears afterwards surfaces.
		// The frozen review-requested snapshot predates both. reviewed_at is kept.
		if err := p.store.ReviewRequests.UpdateReviewers(ctx, repo.ID, rr.PRNumber, marshalReviewers(pr.Reviewers)); err != nil {
			p.logger.Warn("vcs: update reviewers failed",
				zap.Int("pr", rr.PRNumber), zap.Error(err))
		}
		if err := p.store.ReviewRequests.UpdateMergeable(ctx, repo.ID, rr.PRNumber, pr.Mergeable, pr.MergeStateStatus); err != nil {
			p.logger.Warn("vcs: update mergeable failed",
				zap.Int("pr", rr.PRNumber), zap.Error(err))
		}
		// The same snapshot also reveals the user's own decision — typically the
		// approval that removed this PR from the review-requested search in the
		// first place. Authored PRs are skipped: "my review" is meaningless there.
		if myLogin != "" && rr.Kind != "authored" {
			myState := myStateFromReviewers(pr.Reviewers, myLogin)
			if err := p.store.ReviewRequests.UpdateMyReviewState(ctx, repo.ID, rr.PRNumber, myState); err != nil {
				p.logger.Warn("vcs: update my review state failed",
					zap.Int("pr", rr.PRNumber), zap.Error(err))
			}
			// A decisive own review answers a pending re-request: approving is what
			// removed the PR from the review-requested list in the first place, so
			// the frozen "re_review_required" must not outlive it — rewrite it to
			// the user's decision instead of leaving the card stuck until merge.
			if rr.ReviewStatus == "re_review_required" && (myState == "approved" || myState == "changes_requested") {
				if err := p.store.ReviewRequests.SetReviewStatus(ctx, repo.ID, rr.PRNumber, myState); err != nil {
					p.logger.Warn("vcs: clear answered re-review failed",
						zap.Int("pr", rr.PRNumber), zap.Error(err))
				}
			}
		}
	}
}

// myStateFromReviewers picks `login`'s own entry out of a PR's collapsed reviewer
// decisions, returning "" when they hold no standing review on it.
func myStateFromReviewers(ds []vcs.ReviewerDecision, login string) string {
	for _, d := range ds {
		if d.Login == login {
			return d.State
		}
	}
	return ""
}

// marshalReviewers serializes reviewer decisions to the JSON stored on a review
// request row, returning "[]" for an empty/failed marshal.
func marshalReviewers(ds []vcs.ReviewerDecision) string {
	if len(ds) == 0 {
		return "[]"
	}
	out := make([]vcsstore.ReviewerDecision, 0, len(ds))
	for _, d := range ds {
		out = append(out, vcsstore.ReviewerDecision{Login: d.Login, State: d.State, SubmittedAt: d.SubmittedAt})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(b)
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
