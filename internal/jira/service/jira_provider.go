package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/provider"
	"go.uber.org/zap"
)

// JiraProvider implements provider.Provider for Atlassian Jira.
type JiraProvider struct {
	jiraRepo *jirastore.Repo
	jira     JiraAPI
	logger   *zap.Logger
}

// NewJiraProvider constructs a JiraProvider.
func NewJiraProvider(jiraRepo *jirastore.Repo, jiraClient JiraAPI, logger *zap.Logger) *JiraProvider {
	return &JiraProvider{jiraRepo: jiraRepo, jira: jiraClient, logger: logger}
}

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
func (p *JiraProvider) Refresh(ctx context.Context, contextID string) (provider.Stats, error) {
	issues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		return provider.Stats{}, fmt.Errorf("loading issues: %w", err)
	}

	stats := provider.Stats{}

	for _, issue := range issues {
		detail, err := p.jira.ViewIssue(ctx, issue.JiraKey)
		if err != nil {
			p.logger.Warn("jira refresh: view issue failed", zap.String("key", issue.JiraKey), zap.Error(err))
			continue
		}
		_, err = p.jiraRepo.UpsertIssue(ctx, contextID,
			detail.Key, detail.Summary, detail.Status, detail.Assignee,
			detail.IssueType, detail.Description, detail.Updated,
		)
		if err != nil {
			p.logger.Warn("jira refresh: upsert issue failed", zap.String("key", issue.JiraKey), zap.Error(err))
			continue
		}
		stats.ItemsRefreshed++
	}

	newComments, err := p.syncComments(ctx, issues)
	if err != nil {
		p.logger.Warn("jira refresh: sync comments failed", zap.String("contextID", contextID), zap.Error(err))
	}
	stats.NewItems = newComments

	return stats, nil
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

	existingKeys, err := p.jiraRepo.GetIssueJiraKeys(ctx, contextID)
	if err != nil {
		return provider.Stats{}, fmt.Errorf("loading existing keys: %w", err)
	}

	stats := provider.Stats{}
	for _, r := range results {
		if existingKeys[r.Key] {
			continue
		}
		detail, err := p.jira.ViewIssue(ctx, r.Key)
		if err != nil {
			p.logger.Warn("jira reload: view issue failed", zap.String("key", r.Key), zap.Error(err))
			continue
		}
		_, err = p.jiraRepo.UpsertIssue(ctx, contextID,
			detail.Key, detail.Summary, detail.Status, detail.Assignee,
			detail.IssueType, detail.Description, detail.Updated,
		)
		if err != nil {
			p.logger.Warn("jira reload: upsert issue failed", zap.String("key", r.Key), zap.Error(err))
			continue
		}
		stats.ItemsRefreshed++
	}

	// Re-fetch the full list (including newly added issues) for comment sync.
	allIssues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, false)
	if err != nil {
		p.logger.Warn("jira reload: list issues failed", zap.String("contextID", contextID), zap.Error(err))
	} else {
		newComments, err := p.syncComments(ctx, allIssues)
		if err != nil {
			p.logger.Warn("jira reload: sync comments failed", zap.String("contextID", contextID), zap.Error(err))
		}
		stats.NewItems = newComments
	}

	return stats, nil
}

// WriteContext appends Jira issues and their comments for contextID to w.
// When activeOnly is true only active issues and comments are included.
func (p *JiraProvider) WriteContext(ctx context.Context, w io.Writer, contextID string, startAt time.Time, activeOnly bool) error {
	issues, err := p.jiraRepo.GetIssuesByContext(ctx, contextID, activeOnly)
	if err != nil {
		return fmt.Errorf("loading issues: %w", err)
	}

	for i, issue := range issues {
		comments, err := p.jiraRepo.GetCommentsByIssue(ctx, issue.ID, activeOnly)
		if err != nil {
			return fmt.Errorf("loading comments for %s: %w", issue.JiraKey, err)
		}
		issues[i].Comments = comments
	}
	writeJiraSection(w, issues, startAt)
	return nil
}

// syncComments fetches and deduplicates comments for all given issues.
// Returns the count of newly inserted comments.
func (p *JiraProvider) syncComments(ctx context.Context, issues []jirastore.Issue) (int, error) {
	return syncIssueComments(ctx, p.jira, p.jiraRepo, issues, p.logger)
}

// writeJiraSection renders Jira issues as human-readable text into w.
// When startAt is non-zero, an issue is included only if its updated timestamp or
// at least one comment is on or after startAt.
func writeJiraSection(w io.Writer, issues []jirastore.Issue, startAt time.Time) {
	filtered := make([]jirastore.Issue, 0, len(issues))
	for _, issue := range issues {
		if startAt.IsZero() {
			filtered = append(filtered, issue)
			continue
		}
		// Treat unparseable timestamps as "include" to avoid silent data loss.
		t := ParseTime(issue.JiraUpdated)
		if t.IsZero() || !t.Before(startAt) {
			filtered = append(filtered, issue)
			continue
		}
		for _, c := range issue.Comments {
			ct := ParseTime(c.JiraCreated)
			if ct.IsZero() || !ct.Before(startAt) {
				filtered = append(filtered, issue)
				break
			}
		}
	}

	_, _ = fmt.Fprintf(w, "--- JIRA (%d issue%s) ---\n\n", len(filtered), plural(len(filtered)))

	if len(filtered) == 0 {
		_, _ = fmt.Fprint(w, "No Jira issues match the given criteria.\n\n")
		return
	}

	for _, issue := range filtered {
		_, _ = fmt.Fprintf(w, "[%s] %s\n", issue.JiraKey, issue.Summary)
		_, _ = fmt.Fprintf(w, "Status: %s", issue.Status)
		if issue.Assignee != "" {
			_, _ = fmt.Fprintf(w, " | Assignee: %s", issue.Assignee)
		}
		if issue.IssueType != "" {
			_, _ = fmt.Fprintf(w, " | Type: %s", issue.IssueType)
		}
		if issue.JiraUpdated != "" {
			_, _ = fmt.Fprintf(w, " | Updated: %s", formatDateStr(issue.JiraUpdated))
		}
		_, _ = fmt.Fprint(w, "\n")

		if issue.Description != "" {
			_, _ = fmt.Fprint(w, "\nDescription:\n")
			_, _ = fmt.Fprint(w, indent(issue.Description, "  "))
			_, _ = fmt.Fprint(w, "\n")
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
			_, _ = fmt.Fprintf(w, "\nComments (%d):\n", len(comments))
			for _, c := range comments {
				author := c.Author
				if author == "" {
					author = "Unknown"
				}
				_, _ = fmt.Fprintf(w, "  [%s] %s:\n", formatDateStr(c.JiraCreated), author)
				_, _ = fmt.Fprint(w, indent(c.Body, "    "))
				_, _ = fmt.Fprint(w, "\n")
			}
		}

		_, _ = fmt.Fprint(w, "\n")
	}
}
