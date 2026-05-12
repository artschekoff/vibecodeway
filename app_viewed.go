package main

import (
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// MarkIssueViewed marks a Jira issue as viewed by setting viewed_at = now.
func (a *App) MarkIssueViewed(id string) error {
	return a.svc.MarkIssueViewed(a.ctx, id)
}

// MarkCommentViewed marks a Jira comment as viewed.
func (a *App) MarkCommentViewed(id string) error {
	return a.svc.MarkCommentViewed(a.ctx, id)
}

// MarkSlackThreadViewed marks a Slack thread as viewed.
func (a *App) MarkSlackThreadViewed(id string) error {
	return a.svc.MarkSlackThreadViewed(a.ctx, id)
}

// MarkSlackDMConfigViewed marks a Slack DM config as viewed.
func (a *App) MarkSlackDMConfigViewed(id string) error {
	return a.svc.MarkSlackDMConfigViewed(a.ctx, id)
}

// MarkPRViewed marks a VCS pull request as viewed.
func (a *App) MarkPRViewed(id string) error {
	return a.vcsStore.PullRequests.MarkPRViewed(a.ctx, id)
}

// MarkPRCommentViewed marks a VCS PR comment as viewed.
func (a *App) MarkPRCommentViewed(id string) error {
	return a.vcsStore.PullRequests.MarkPRCommentViewed(a.ctx, id)
}

// MarkBranchAllViewed marks all PRs and PR comments for the given branch as viewed.
func (a *App) MarkBranchAllViewed(id string) error {
	return a.vcsStore.PullRequests.MarkBranchAllViewed(a.ctx, id)
}

// MarkSlackChannelAllViewed marks all threads under a Slack channel as viewed.
func (a *App) MarkSlackChannelAllViewed(id string) error {
	return a.svc.MarkSlackChannelAllViewed(a.ctx, id)
}

// MarkIssueAllViewed marks a Jira issue and all its comments as viewed.
func (a *App) MarkIssueAllViewed(id string) error {
	return a.svc.MarkIssueAllViewed(a.ctx, id)
}

// MarkContextAllViewed sets viewed_at = now for every unread item in a context.
func (a *App) MarkContextAllViewed(contextID string) error {
	return a.svc.MarkContextAllViewed(a.ctx, contextID)
}

// GetUnviewedBreakdown returns per-type unread counts for a context.
func (a *App) GetUnviewedBreakdown(contextID string) (appstore.UnviewedBreakdown, error) {
	return a.svc.GetUnviewedBreakdown(a.ctx, contextID)
}

// SetJiraBaseURL stores the Jira instance base URL for a context.
func (a *App) SetJiraBaseURL(contextID, baseURL string) error {
	return a.svc.SetJiraBaseURL(a.ctx, contextID, baseURL)
}

// RefreshAllContexts re-fetches all active contexts from external sources.
// Progress events ("reload:progress") are emitted via the Wails runtime before each provider step,
// using the same schema as ReloadAllContexts so the frontend can share one handler.
func (a *App) RefreshAllContexts() ([]jiraservice.ReloadAllResult, error) {
	return a.svc.RefreshAllContexts(a.ctx, func(current, total int, contextKey, step string) {
		runtime.EventsEmit(a.ctx, "reload:progress", map[string]interface{}{
			"current":    current,
			"total":      total,
			"contextKey": contextKey,
			"step":       step,
		})
	})
}
