package main

import (
	"fmt"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
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
		})
	}
	if err := a.db.UpsertBoardJiraAssignments(a.ctx, issues); err != nil {
		return nil, fmt.Errorf("persist assigned jira: %w", err)
	}
	return a.db.ListBoardJiraAssignments(a.ctx, orderBy)
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
