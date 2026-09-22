package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artschekoff/vibecodeway/internal/jira"
	jirastore "github.com/artschekoff/vibecodeway/internal/jira/store"
	"github.com/artschekoff/vibecodeway/internal/provider"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"go.uber.org/zap"
)

// JiraNotifyFunc is called when a tracked Jira issue changes status.
// Same signature as vcsservice.NotifyFunc so app.go can share a factory.
type JiraNotifyFunc func(ctx context.Context, contextID, entityType, entityID, eventType, oldValue, newValue string) error

// JiraRecordStatFunc persists one analytics-dashboard stat event. App wiring
// owns the actual store write, mirroring vcsservice.RecordStatFunc — see
// WithRecordStatFunc.
type JiraRecordStatFunc func(ctx context.Context, e appstore.StatEvent) error

// JiraStatsIdentityFunc resolves "who am I" for comment_authored attribution.
// It is called live on every sync (not cached at construction time) so a
// change to the underlying app setting takes effect without restarting the
// provider. An empty return means capture should be skipped — see
// WithStatsIdentityFunc.
type JiraStatsIdentityFunc func(ctx context.Context) (string, error)

// JiraProvider implements provider.Provider for Atlassian Jira.
type JiraProvider struct {
	jiraRepo      *jirastore.Repo
	jira          JiraAPI
	logger        *zap.Logger
	msgWriter     appstore.MessageWriter
	notify        JiraNotifyFunc
	recordStat    JiraRecordStatFunc
	statsIdentity JiraStatsIdentityFunc
}

// NewJiraProvider constructs a JiraProvider.
func NewJiraProvider(jiraRepo *jirastore.Repo, jiraClient JiraAPI, logger *zap.Logger, msgWriter appstore.MessageWriter) *JiraProvider {
	return &JiraProvider{jiraRepo: jiraRepo, jira: jiraClient, logger: logger, msgWriter: msgWriter}
}

// WithNotifyFunc sets the callback invoked on Jira issue status changes.
func (p *JiraProvider) WithNotifyFunc(fn JiraNotifyFunc) { p.notify = fn }

// WithRecordStatFunc sets the callback invoked to persist a stat event.
func (p *JiraProvider) WithRecordStatFunc(fn JiraRecordStatFunc) { p.recordStat = fn }

// WithStatsIdentityFunc sets the callback used to resolve the current user's
// Jira display name for comment_authored attribution.
func (p *JiraProvider) WithStatsIdentityFunc(fn JiraStatsIdentityFunc) { p.statsIdentity = fn }

// Name implements provider.Provider.
func (p *JiraProvider) Name() string { return "jira" }

// IsConfigured implements provider.Provider.
func (p *JiraProvider) IsConfigured(ctx context.Context, contextID string) (bool, error) {
	_, err := p.jiraRepo.GetJiraProvider(ctx, contextID)
	if errors.Is(err, jirastore.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Refresh re-fetches field values and comments for every issue already stored in contextID.
// It does NOT run the JQL query or add new issues.
// Stats.ItemsRefreshed = issues updated; Stats.NewItems = new comments inserted.
// upsertWithMessage persists an issue detail and mirrors it into the message store.
// Shared by Refresh's existing-issue pass and its newly-discovered-children pass.
func (p *JiraProvider) upsertWithMessage(ctx context.Context, contextID string, detail *jira.IssueDetail) (jirastore.IssueUpsertResult, error) {
	res, err := p.jiraRepo.UpsertIssue(
		ctx, contextID,
		detail.Key, detail.Summary, detail.Status, detail.Assignee,
		detail.IssueType, detail.Description, detail.Updated, detail.ParentKey,
		detail.Priority,
	)
	if err != nil {
		return res, err
	}
	if p.msgWriter != nil {
		authorID := p.msgWriter.ResolveAuthor(ctx, "jira", detail.Assignee)
		msgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
			ContextID:  contextID,
			SourceID:   "jira",
			ExternalID: detail.Key,
			Title:      detail.Summary,
			Body:       detail.Description,
			RawAuthor:  detail.Assignee,
			AuthorID:   authorID,
			CreatedAt:  detail.Updated,
			UpdatedAt:  detail.Updated,
		})
		if msgID != "" {
			_ = p.jiraRepo.SetIssueMessageID(ctx, res.ID, msgID)
		}
	}
	return res, nil
}

func (p *JiraProvider) Refresh(ctx context.Context, contextID string) (provider.Stats, error) {
	return p.refresh(ctx, contextID, nil)
}

// RefreshWithProgress implements provider.ProgressReporter: report gets one
// call per issue being fetched ("JH-1234 5/99"), from concurrent workers.
func (p *JiraProvider) RefreshWithProgress(ctx context.Context, contextID string, report provider.ProgressFunc) (provider.Stats, error) {
	return p.refresh(ctx, contextID, report)
}

func (p *JiraProvider) refresh(ctx context.Context, contextID string, report provider.ProgressFunc) (provider.Stats, error) {
	issues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		return provider.Stats{}, fmt.Errorf("loading issues: %w", err)
	}

	if len(issues) == 0 {
		return provider.Stats{}, nil
	}

	storedKeys := make(map[string]bool, len(issues))
	for _, iss := range issues {
		storedKeys[iss.JiraKey] = true
	}
	var childMu sync.Mutex
	childCandidates := make(map[string]bool)

	workers := min(5, len(issues))
	jobs := make(chan jirastore.Issue, len(issues))
	for _, iss := range issues {
		jobs <- iss
	}
	close(jobs)

	var refreshed atomic.Int64
	var notifications atomic.Int64
	var started atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for issue := range jobs {
				if report != nil {
					report(fmt.Sprintf("%s %d/%d", issue.JiraKey, started.Add(1), len(issues)))
				}
				detail, err := p.jira.ViewIssue(ctx, issue.JiraKey)
				if err != nil {
					p.logger.Warn("jira refresh: view issue failed", zap.String("key", issue.JiraKey), zap.Error(err))
					continue
				}
				res, err := p.upsertWithMessage(ctx, contextID, detail)
				if err != nil {
					p.logger.Warn("jira refresh: upsert issue failed", zap.String("key", issue.JiraKey), zap.Error(err))
					continue
				}
				if res.StatusChanged && p.notify != nil {
					if notifyErr := p.notify(ctx, contextID, "jira_issue", res.ID,
						"jira_status_changed", res.OldStatus, detail.Status); notifyErr != nil {
						p.logger.Warn("jira refresh: status notification failed",
							zap.String("key", detail.Key), zap.Error(notifyErr))
					} else {
						notifications.Add(1)
					}
				}
				// Collect new subtasks so children created after the initial add are
				// pulled in on refresh (matches AddIssuesToContext discovery).
				if len(detail.Subtasks) > 0 {
					childMu.Lock()
					for _, sub := range detail.Subtasks {
						if !storedKeys[sub.Key] {
							childCandidates[sub.Key] = true
						}
					}
					childMu.Unlock()
				}
				refreshed.Add(1)
			}
		}()
	}
	wg.Wait()

	// Next-gen Jira projects don't list children in the subtasks field; find them
	// via JQL. ponytail: one query per stored issue, serial — contexts hold a handful.
	for key := range storedKeys {
		children, err := p.jira.Search(ctx, fmt.Sprintf("parent = %q", key))
		if err != nil {
			p.logger.Warn("jira refresh: child search failed", zap.String("key", key), zap.Error(err))
			continue
		}
		for _, ch := range children {
			if !storedKeys[ch.Key] {
				childCandidates[ch.Key] = true
			}
		}
	}

	// Upsert newly discovered children (one level deep, like AddIssuesToContext).
	// TouchContext isn't needed here; RefreshContext tags these as new via the sync session.
	for key := range childCandidates {
		detail, err := p.jira.ViewIssue(ctx, key)
		if err != nil {
			p.logger.Warn("jira refresh: view child failed", zap.String("key", key), zap.Error(err))
			continue
		}
		if _, err := p.upsertWithMessage(ctx, contextID, detail); err != nil {
			p.logger.Warn("jira refresh: upsert child failed", zap.String("key", key), zap.Error(err))
			continue
		}
		refreshed.Add(1)
	}

	newComments, err := p.syncComments(ctx, contextID, issues)
	if err != nil {
		p.logger.Warn("jira refresh: sync comments failed", zap.String("contextID", contextID), zap.Error(err))
	}
	return provider.Stats{
		ItemsRefreshed:   int(refreshed.Load()),
		NewItems:         newComments,
		NewNotifications: int(notifications.Load()),
	}, nil
}

// Reload runs the context's saved JQL query, adds newly found issues, then refreshes all.
// Stats.ItemsRefreshed = new issues added from JQL; Stats.NewItems = new comments inserted.
func (p *JiraProvider) Reload(ctx context.Context, contextID string) (provider.Stats, error) {
	jp, err := p.jiraRepo.GetJiraProvider(ctx, contextID)
	if errors.Is(err, jirastore.ErrNotFound) {
		// No JQL configured — just refresh existing issues.
		return p.Refresh(ctx, contextID)
	}
	if err != nil {
		return provider.Stats{}, fmt.Errorf("loading jira config: %w", err)
	}

	results, err := p.jira.Search(ctx, jp.SearchQuery)
	if err != nil {
		p.logger.Warn("jira reload: search failed", zap.String("contextID", contextID), zap.Error(err))
	}

	// Load stored issues to detect status changes on existing ones.
	storedIssues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		return provider.Stats{}, fmt.Errorf("loading existing issues: %w", err)
	}
	existingByKey := make(map[string]*jirastore.Issue, len(storedIssues))
	for i := range storedIssues {
		existingByKey[storedIssues[i].JiraKey] = &storedIssues[i]
	}

	stats := provider.Stats{}
	for _, r := range results {
		if stored, exists := existingByKey[r.Key]; exists {
			// Existing issue: check if status changed using search result status.
			if r.Status != stored.Status {
				res, uErr := p.jiraRepo.UpsertIssue(ctx, contextID,
					r.Key, r.Summary, r.Status, r.Assignee, r.IssueType,
					stored.Description, stored.JiraUpdated, stored.ParentKey, stored.Priority)
				if uErr != nil {
					p.logger.Warn("jira reload: update status failed", zap.String("key", r.Key), zap.Error(uErr))
				} else if res.StatusChanged && p.notify != nil {
					if notifyErr := p.notify(ctx, contextID, "jira_issue", res.ID,
						"jira_status_changed", res.OldStatus, r.Status); notifyErr != nil {
						p.logger.Warn("jira reload: status notification failed", zap.Error(notifyErr))
					} else {
						stats.NewNotifications++
					}
				}
			}
			continue
		}
		// New issue: fetch full details.
		detail, err := p.jira.ViewIssue(ctx, r.Key)
		if err != nil {
			p.logger.Warn("jira reload: view issue failed", zap.String("key", r.Key), zap.Error(err))
			continue
		}
		reloadRes, err := p.jiraRepo.UpsertIssue(
			ctx, contextID,
			detail.Key, detail.Summary, detail.Status, detail.Assignee,
			detail.IssueType, detail.Description, detail.Updated, detail.ParentKey,
			detail.Priority,
		)
		if err != nil {
			p.logger.Warn("jira reload: upsert issue failed", zap.String("key", r.Key), zap.Error(err))
			continue
		}
		if p.msgWriter != nil {
			authorID := p.msgWriter.ResolveAuthor(ctx, "jira", detail.Assignee)
			msgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
				ContextID:  contextID,
				SourceID:   "jira",
				ExternalID: detail.Key,
				Title:      detail.Summary,
				Body:       detail.Description,
				RawAuthor:  detail.Assignee,
				AuthorID:   authorID,
				CreatedAt:  detail.Updated,
				UpdatedAt:  detail.Updated,
			})
			if msgID != "" {
				_ = p.jiraRepo.SetIssueMessageID(ctx, reloadRes.ID, msgID)
			}
		}
		stats.ItemsRefreshed++
	}

	// Re-fetch the full list (including newly added issues) for comment sync.
	allIssues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		p.logger.Warn("jira reload: list issues failed", zap.String("contextID", contextID), zap.Error(err))
	} else {
		newComments, err := p.syncComments(ctx, contextID, allIssues)
		if err != nil {
			p.logger.Warn("jira reload: sync comments failed", zap.String("contextID", contextID), zap.Error(err))
		}
		stats.NewItems = newComments
	}

	return stats, nil
}

// CollectItems returns one Item per Jira issue for global sorting.
func (p *JiraProvider) CollectItems(ctx context.Context, contextID string, startAt time.Time, activeOnly bool) ([]provider.Item, error) {
	issues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("loading issues: %w", err)
	}
	for i, issue := range issues {
		comments, err := p.jiraRepo.GetCommentsByIssue(ctx, issue.ID, activeOnly)
		if err != nil {
			return nil, fmt.Errorf("loading comments for %s: %w", issue.JiraKey, err)
		}
		issues[i].Comments = comments
	}
	return buildJiraItems(issues, startAt), nil
}

// syncComments fetches and deduplicates comments for all given issues and, when a
// MessageWriter is configured, writes a message record for each newly inserted comment.
// Returns the count of newly inserted comments.
func (p *JiraProvider) syncComments(ctx context.Context, contextID string, issues []jirastore.Issue) (int, error) {
	if p.msgWriter == nil {
		return syncIssueComments(ctx, p.jira, p.jiraRepo, issues, p.logger)
	}

	if len(issues) == 0 {
		return 0, nil
	}

	workers := min(5, len(issues))
	jobs := make(chan jirastore.Issue, len(issues))
	for _, iss := range issues {
		jobs <- iss
	}
	close(jobs)

	var total atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for issue := range jobs {
				comments, err := p.jira.ListComments(ctx, issue.JiraKey)
				if err != nil {
					p.logger.Warn("sync comments: list failed", zap.String("key", issue.JiraKey), zap.Error(err))
					continue
				}

				var issueMsgID string
				if issue.MessageID != nil {
					issueMsgID = *issue.MessageID
				}

				for _, c := range comments {
					commentID, err := p.jiraRepo.UpsertComment(ctx, issue.ID, c.ID, c.Author, c.Body, c.Created)
					if err != nil {
						p.logger.Warn("sync comments: upsert failed",
							zap.String("issue", issue.JiraKey), zap.String("jiraID", c.ID), zap.Error(err))
						continue
					}
					if commentID != "" {
						total.Add(1)
						p.recordCommentStat(ctx, issue.JiraKey, c)
						if issueMsgID != "" {
							cAuthorID := p.msgWriter.ResolveAuthor(ctx, "jira", c.Author)
							cMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
								ContextID:  contextID,
								SourceID:   "jira",
								ExternalID: c.ID,
								ParentID:   issueMsgID,
								Body:       c.Body,
								RawAuthor:  c.Author,
								AuthorID:   cAuthorID,
								CreatedAt:  c.Created,
								UpdatedAt:  c.Created,
							})
							if cMsgID != "" {
								_ = p.jiraRepo.SetCommentMessageID(ctx, commentID, cMsgID)
							}
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	return int(total.Load()), nil
}

// recordCommentStat best-effort records a comment_authored stat event for a
// newly-synced comment c, when both callbacks are wired and c.Author matches
// the identity resolved by statsIdentity. Never returns an error and never
// breaks comment sync — failures are warn-logged and swallowed, matching the
// rest of syncComments' error handling.
func (p *JiraProvider) recordCommentStat(ctx context.Context, jiraKey string, c jira.Comment) {
	if p.recordStat == nil || p.statsIdentity == nil {
		return
	}
	me, err := p.statsIdentity(ctx)
	if err != nil {
		p.logger.Warn("jira: resolve stats identity failed", zap.Error(err))
		return
	}
	// Empty identity means the user never configured statsAuthorName: skip
	// capture entirely, silently (see task-3 brief).
	if me == "" || c.Author == "" || !strings.EqualFold(me, c.Author) {
		return
	}
	occurredAt := c.Created
	if t := ParseTime(c.Created); !t.IsZero() {
		occurredAt = t.UTC().Format(time.RFC3339)
	}
	if err := p.recordStat(ctx, appstore.StatEvent{
		Kind:       "comment_authored",
		ExternalID: c.ID,
		JiraKey:    jiraKey,
		OccurredAt: occurredAt,
	}); err != nil {
		p.logger.Warn("jira: record stat event failed", zap.Error(err))
	}
}

func buildJiraItems(issues []jirastore.Issue, startAt time.Time) []provider.Item {
	var items []provider.Item
	for _, issue := range issues {
		// Apply startAt filter: include if issue or any comment is on/after startAt.
		if !startAt.IsZero() {
			t := ParseTime(issue.JiraUpdated)
			inWindow := t.IsZero() || !t.Before(startAt)
			if !inWindow {
				for _, c := range issue.Comments {
					ct := ParseTime(c.JiraCreated)
					if ct.IsZero() || !ct.Before(startAt) {
						inWindow = true
						break
					}
				}
			}
			if !inWindow {
				continue
			}
		}

		ts := ParseTime(issue.JiraUpdated)
		if ts.IsZero() {
			ts = ParseTime(issue.CreatedAt)
		}

		var buf strings.Builder
		fmt.Fprintf(&buf, "[%s · JIRA] [%s] %s\n", FormatDateTime(issue.JiraUpdated), issue.JiraKey, issue.Summary)
		fmt.Fprintf(&buf, "Status: %s", issue.Status)
		if issue.Assignee != "" {
			fmt.Fprintf(&buf, " | Assignee: %s", issue.Assignee)
		}
		if issue.IssueType != "" {
			fmt.Fprintf(&buf, " | Type: %s", issue.IssueType)
		}
		fmt.Fprint(&buf, "\n")

		if issue.Description != "" {
			fmt.Fprint(&buf, "\nDescription:\n")
			fmt.Fprint(&buf, indent(issue.Description, "  "))
			fmt.Fprint(&buf, "\n")
		}

		comments := issue.Comments
		if !startAt.IsZero() {
			var fc []jirastore.Comment
			for _, c := range comments {
				if !ParseTime(c.JiraCreated).Before(startAt) {
					fc = append(fc, c)
				}
			}
			comments = fc
		}
		if len(comments) > 0 {
			fmt.Fprintf(&buf, "\nComments (%d):\n", len(comments))
			for _, c := range comments {
				author := c.Author
				if author == "" {
					author = "Unknown"
				}
				fmt.Fprintf(&buf, "  [%s] %s:\n", FormatDateTime(c.JiraCreated), author)
				fmt.Fprint(&buf, indent(c.Body, "    "))
				fmt.Fprint(&buf, "\n")
			}
		}
		fmt.Fprint(&buf, "\n")

		items = append(items, provider.Item{CreatedAt: ts, Body: buf.String()})
	}
	return items
}
