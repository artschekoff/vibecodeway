package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	"go.uber.org/zap"
)

// instanceSiteRE extracts the bare hostname from the "Site:" line of acli jira auth status output.
var instanceSiteRE = regexp.MustCompile(`(?i)Site:\s*(\S+)`)

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

// GetInstanceURL extracts the Jira instance URL from the auth status output.
func (c *Client) GetInstanceURL(ctx context.Context) string {
	out, err := c.AuthStatus(ctx)
	if err != nil {
		c.logger.Debug("GetInstanceURL: auth status failed", zap.Error(err))
		return ""
	}
	m := instanceSiteRE.FindStringSubmatch(out)
	if len(m) < 2 {
		return ""
	}
	host := strings.TrimRight(m[1], "/.")
	return "https://" + host
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
		"--fields", "key,issuetype,summary,status,assignee,description,updated,subtasks,issuelinks",
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
	detail := &IssueDetail{
		Key:         item.key(),
		Summary:     item.field("summary"),
		Status:      item.field("status"),
		Assignee:    item.field("assignee"),
		IssueType:   item.field("issuetype"),
		Description: extractADFText(item.Fields["description"]),
		Updated:     item.field("updated"),
	}
	if rawSubs := item.Fields["subtasks"]; rawSubs != nil {
		var subs []rawSubtask
		if json.Unmarshal(rawSubs, &subs) == nil {
			for _, s := range subs {
				detail.Subtasks = append(detail.Subtasks, SubtaskRef{
					Key:     s.Key,
					Summary: parseString(s.Fields["summary"]),
					Status:  parseString(s.Fields["status"]),
				})
			}
		}
	}
	if rawLinks := item.Fields["issuelinks"]; rawLinks != nil {
		var links []rawIssueLink
		if json.Unmarshal(rawLinks, &links) == nil {
			for _, l := range links {
				typeName := parseString(l.Type["name"])
				if l.OutwardIssue != nil && l.OutwardIssue.Key != "" {
					detail.Links = append(detail.Links, IssueLink{
						TypeName:  typeName,
						Direction: "outward",
						LinkedKey: l.OutwardIssue.Key,
					})
				}
				if l.InwardIssue != nil && l.InwardIssue.Key != "" {
					detail.Links = append(detail.Links, IssueLink{
						TypeName:  typeName,
						Direction: "inward",
						LinkedKey: l.InwardIssue.Key,
					})
				}
			}
		}
	}
	return detail, nil
}

// rawSubtask mirrors the simplified issue reference in the Jira subtasks array.
type rawSubtask struct {
	Key    string                     `json:"key"`
	Fields map[string]json.RawMessage `json:"fields"`
}

// rawIssueLink mirrors the Jira REST API issuelinks array element.
type rawIssueLink struct {
	Type         map[string]json.RawMessage `json:"type"`
	InwardIssue  *rawIssueLinkRef           `json:"inwardIssue"`
	OutwardIssue *rawIssueLinkRef           `json:"outwardIssue"`
}

type rawIssueLinkRef struct {
	Key string `json:"key"`
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
