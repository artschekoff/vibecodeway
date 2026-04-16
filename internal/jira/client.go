package jira

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	"go.uber.org/zap"
)

// Client wraps the acli Runner and provides typed Jira query methods.
type Client struct {
	runner *acli.Runner
	logger *zap.Logger
}

// NewClient creates a new Jira client.
func NewClient(runner *acli.Runner, logger *zap.Logger) *Client {
	return &Client{runner: runner, logger: logger}
}

// AuthStatus returns the current acli authentication status string.
func (c *Client) AuthStatus(ctx context.Context) (string, error) {
	return c.runner.Run(ctx, []string{"jira", "auth", "status"}, nil, nil)
}

// Search executes a JQL search and returns matching issues.
func (c *Client) Search(ctx context.Context, jql string) ([]SearchResult, error) {
	flags := []string{
		"--jql", jql,
		"--fields", "key,summary,status,assignee,issuetype",
		"--json",
	}
	out, err := c.runner.Run(ctx, []string{"jira", "workitem", "search"}, nil, flags)
	if err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}
	return parseSearchResults(out)
}

// ViewIssue fetches full details for a single issue key.
func (c *Client) ViewIssue(ctx context.Context, key string) (*IssueDetail, error) {
	flags := []string{
		"--fields", "key,issuetype,summary,status,assignee,description,updated",
		"--json",
	}
	out, err := c.runner.Run(ctx, []string{"jira", "workitem", "view"}, []string{key}, flags)
	if err != nil {
		return nil, fmt.Errorf("jira view %s: %w", key, err)
	}
	return parseIssueDetail(out)
}

// ListComments returns all comments for an issue.
func (c *Client) ListComments(ctx context.Context, key string) ([]Comment, error) {
	flags := []string{
		"--key", key,
		"--json",
	}
	out, err := c.runner.Run(ctx, []string{"jira", "workitem", "comment", "list"}, nil, flags)
	if err != nil {
		return nil, fmt.Errorf("jira comment list %s: %w", key, err)
	}
	return parseComments(out)
}

// acliIssue mirrors the Jira REST API envelope returned by acli --json.
// Top-level has "key"; all other issue fields live under "fields".
type acliIssue struct {
	Key    json.RawMessage            `json:"key"`
	Fields map[string]json.RawMessage `json:"fields"`
}

func (a acliIssue) key() string { return parseString(a.Key) }
func (a acliIssue) field(k string) string {
	return parseString(a.Fields[k])
}

func parseSearchResults(raw string) ([]SearchResult, error) {
	var items []acliIssue
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	results := make([]SearchResult, 0, len(items))
	for _, item := range items {
		results = append(results, SearchResult{
			Key:       item.key(),
			Summary:   item.field("summary"),
			Status:    item.field("status"),
			Assignee:  item.field("assignee"),
			IssueType: item.field("issuetype"),
		})
	}
	return results, nil
}

func parseIssueDetail(raw string) (*IssueDetail, error) {
	var item acliIssue
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return nil, fmt.Errorf("parsing issue detail: %w", err)
	}
	return &IssueDetail{
		Key:         item.key(),
		Summary:     item.field("summary"),
		Status:      item.field("status"),
		Assignee:    item.field("assignee"),
		IssueType:   item.field("issuetype"),
		Description: extractADFText(item.Fields["description"]),
		Updated:     item.field("updated"),
	}, nil
}

// acliCommentList mirrors the paginated comment list response from acli --json.
type acliCommentList struct {
	Comments []rawIssue `json:"comments"`
}

func parseComments(raw string) ([]Comment, error) {
	var resp acliCommentList
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parsing comments: %w", err)
	}
	comments := make([]Comment, 0, len(resp.Comments))
	for _, item := range resp.Comments {
		comments = append(comments, Comment{
			ID:      item.str("id"),
			Author:  item.str("author"),
			Body:    item.str("body"),
			Created: item.str("created"),
		})
	}
	return comments, nil
}
