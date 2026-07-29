package main

import (
	"fmt"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

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
