package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
			detail.IssueType, detail.Description, detail.Updated, detail.ParentKey,
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
			detail.IssueType, detail.Description, detail.Updated, detail.ParentKey,
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

// syncComments fetches and deduplicates comments for all given issues.
// Returns the count of newly inserted comments.
func (p *JiraProvider) syncComments(ctx context.Context, issues []jirastore.Issue) (int, error) {
	return syncIssueComments(ctx, p.jira, p.jiraRepo, issues, p.logger)
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
