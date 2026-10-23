package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/artschekoff/vibecodeway/internal/jira"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// CreateContextFromJiraIssue creates a new context (keyed by the Jira issue key)
// in the given org and attaches the issue to it, so a board to-do item becomes a
// tracked context. Returns the new context (its id/key let the board link to it).
func (a *App) CreateContextFromJiraIssue(orgID, jiraKey string) (*appstore.Context, error) {
	if orgID == "" {
		return nil, fmt.Errorf("orgID is required to create a context")
	}
	c, err := a.cliRunner.CreateContext(a.ctx, jiraKey, orgID)
	if err != nil {
		return nil, fmt.Errorf("create context for %s: %w", jiraKey, err)
	}
	if _, err := a.AddIssuesToContext(c.ID, []string{jiraKey}); err != nil {
		return c, fmt.Errorf("context created but attaching %s failed: %w", jiraKey, err)
	}
	return c, nil
}

// RefreshMyAssignedJiraIssues fetches the current user's assigned Jira issues,
// persists them (preserving the seen flag for known keys), and returns the list
// sorted by orderBy ("priority" default, or "created").
func (a *App) RefreshMyAssignedJiraIssues(orderBy string) ([]appstore.BoardJiraAssignment, error) {
	const jql = "assignee = currentUser() ORDER BY priority DESC, created DESC"
	results, err := a.svc.SearchJQL(a.ctx, jql)
	if err != nil {
		return nil, fmt.Errorf("refresh assigned jira: %w", err)
	}
	parents := a.parentsOfMySubtasks(results)
	parentKeys := make(map[string]bool, len(parents))
	for _, p := range parents {
		parentKeys[p.Key] = true
	}
	results = append(results, parents...)
	issues := make([]appstore.BoardJiraAssignment, 0, len(results))
	for _, r := range results {
		issues = append(issues, appstore.BoardJiraAssignment{
			Key:       r.Key,
			Summary:   r.Summary,
			Status:    r.Status,
			IssueType: r.IssueType,
			Priority:  r.Priority,
			Created:   r.Created,
			Org:       r.Org,
			// Parents ride along for visibility; their transitions are someone
			// else's work and must not enter the productivity stats.
			NotMine: parentKeys[r.Key],
		})
	}
	if err := a.db.UpsertBoardJiraAssignments(a.ctx, issues); err != nil {
		return nil, fmt.Errorf("persist assigned jira: %w", err)
	}
	go a.backfillBoardOrgs()
	return a.db.ListBoardJiraAssignments(a.ctx, orderBy)
}

// isSubtaskType matches Jira's subtask issue type across spellings
// ("Subtask", "Sub-task", "Sub task").
func isSubtaskType(t string) bool {
	s := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(t))
	return s == "subtask"
}

// parentsOfMySubtasks returns board rows for parent tickets of the user's
// assigned subtasks — a ticket whose subtask is mine belongs on the board even
// when the ticket itself is assigned to someone else. Parent keys resolve via
// one `workitem view` per subtask, cached permanently (a subtask's parent never
// changes); parent details then come from a single `key in (...)` search.
// Best-effort: any failure just leaves parents out of this refresh.
func (a *App) parentsOfMySubtasks(results []jira.SearchResult) []jira.SearchResult {
	present := make(map[string]bool, len(results))
	var subtaskKeys []string
	for _, r := range results {
		present[r.Key] = true
		if isSubtaskType(r.IssueType) {
			subtaskKeys = append(subtaskKeys, r.Key)
		}
	}
	if len(subtaskKeys) == 0 {
		return nil
	}
	cached, err := a.db.GetSubtaskParents(a.ctx, subtaskKeys)
	if err != nil {
		a.logger.Warn("board: load subtask parents failed", zap.Error(err))
		cached = map[string]string{}
	}
	for _, key := range subtaskKeys {
		if _, ok := cached[key]; ok {
			continue
		}
		parent, err := a.svc.SubtaskParent(a.ctx, key)
		if err != nil {
			// Not cached — retried on the next refresh.
			a.logger.Warn("board: resolve subtask parent failed", zap.String("key", key), zap.Error(err))
			continue
		}
		if parent == "" {
			continue
		}
		cached[key] = parent
		if err := a.db.SetSubtaskParent(a.ctx, key, parent); err != nil {
			a.logger.Warn("board: cache subtask parent failed", zap.String("key", key), zap.Error(err))
		}
	}
	var parentKeys []string
	seen := map[string]bool{}
	for _, parent := range cached {
		if !present[parent] && !seen[parent] {
			seen[parent] = true
			parentKeys = append(parentKeys, parent)
		}
	}
	if len(parentKeys) == 0 {
		return nil
	}
	parents, err := a.svc.SearchJQL(a.ctx,
		fmt.Sprintf("key in (%s) ORDER BY priority DESC, created DESC", strings.Join(parentKeys, ", ")))
	if err != nil {
		a.logger.Warn("board: fetch subtask parents failed", zap.Error(err))
		return nil
	}
	return parents
}

// orgBackfillWorkers caps concurrent `acli workitem view` calls. Each takes ~5s
// and acli spawns a process per call, so this trades wall-clock for not
// saturating the machine while the user works.
const orgBackfillWorkers = 4

// backfillBoardOrgs fills in the org/scope of board assignments one key at a
// time, because a Jira search can't return custom fields (see jira.searchFields)
// and `workitem view` only takes one key per ~5s call. Runs in the background so
// the board renders immediately from cache; results are permanent, so this is
// slow once and then a no-op. Emits "board:orgs-updated" when anything changed.
func (a *App) backfillBoardOrgs() {
	if !a.orgBackfillRunning.CompareAndSwap(false, true) {
		return // a backfill is already walking the queue
	}
	defer a.orgBackfillRunning.Store(false)
	if a.jiraClient == nil || a.jiraClient.OrgField() == "" {
		return // no scope field configured — nothing to look up
	}

	keys, err := a.db.ListBoardKeysMissingOrg(a.ctx)
	if err != nil {
		a.logger.Warn("board org backfill: list keys failed", zap.Error(err))
		return
	}
	if len(keys) == 0 {
		return
	}
	a.logger.Info("board org backfill: starting", zap.Int("keys", len(keys)))

	jobs := make(chan string)
	var wg sync.WaitGroup
	var updated atomic.Int64
	var done atomic.Int64
	total := len(keys)

	for range min(orgBackfillWorkers, len(keys)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobs {
				org, err := a.svc.IssueOrg(a.ctx, key)
				// Progress counts attempts (success or not) so the bar always
				// reaches total instead of stalling on transient acli failures.
				runtime.EventsEmit(a.ctx, "board:backfill-progress", map[string]any{
					"current": done.Add(1), "total": total, "key": key,
				})
				if err != nil {
					// Leave the key queued: a transient acli failure should be
					// retried on the next refresh, not cached as "no org".
					a.logger.Warn("board org backfill: lookup failed",
						zap.String("key", key), zap.Error(err))
					continue
				}
				if err := a.db.SetBoardAssignmentOrg(a.ctx, key, org); err != nil {
					a.logger.Warn("board org backfill: store failed",
						zap.String("key", key), zap.Error(err))
					continue
				}
				updated.Add(1)
			}
		}()
	}
	for _, key := range keys {
		select {
		case <-a.ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- key:
		}
	}
	close(jobs)
	wg.Wait()

	a.logger.Info("board org backfill: done", zap.Int64("updated", updated.Load()))
	if updated.Load() > 0 && a.emit != nil {
		a.emit("board:orgs-updated")
	}
}

// ListMyAssignedJiraIssues returns the persisted assigned Jira issues sorted by
// orderBy ("priority" default, or "created").
func (a *App) ListMyAssignedJiraIssues(orderBy string) ([]appstore.BoardJiraAssignment, error) {
	return a.db.ListBoardJiraAssignments(a.ctx, orderBy)
}

// MarkMyAssignedJiraIssuesSeen marks all assigned Jira issues as seen (clears the
// "new" badge in the Activity Feed).
func (a *App) MarkMyAssignedJiraIssuesSeen() error {
	return a.db.MarkBoardJiraAssignmentsSeen(a.ctx)
}
