package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/jira/acli"
	"go.uber.org/zap"
)

// instanceSiteRE extracts the bare hostname from the "Site:" line of acli jira auth status output.
var instanceSiteRE = regexp.MustCompile(`(?i)Site:\s*(\S+)`)

// OrgScopeFieldID is the Jira custom field this instance uses for the per-ticket
// scope/service (values like "juna-api", "juna-frontend", "rdbms") — treated as
// the ticket's "organization" for the board split.
// ponytail: hardcoded field id (instance-specific); move to a setting if the app
// ever targets multiple Jira instances.
const OrgScopeFieldID = "customfield_10160"

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

// searchFields is the field list passed to `acli jira workitem search`.
// acli enforces a fixed allowlist here and rejects anything outside it with
// "field 'X' is not allowed", which fails the WHOLE search — so this list must
// never grow a custom field (customfield_*) or "created". Custom fields are only
// retrievable via `acli jira workitem view` (see ViewIssue / OrgScopeFieldID).
const searchFields = "key,summary,status,assignee,issuetype,priority"

// Search executes a JQL search and returns matching issues.
func (c *Client) Search(ctx context.Context, jql string) ([]SearchResult, error) {
	flags := []string{
		"--jql", jql,
		// See searchFields: Created and Org stay "" for search results because
		// acli's search allowlist can't return them.
		"--fields", searchFields,
		// Without --paginate acli silently caps at ~30 results; with an ORDER BY
		// that put low-priority items last, whole statuses (To Do) never arrived.
		"--paginate",
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
		"--fields", "key,issuetype,summary,status,assignee,description,updated,subtasks,issuelinks,parent,priority",
		"--json",
	}
	out, err := c.runner.Run(ctx, []string{"jira", "workitem", "view"}, []string{key}, flags)
	if err != nil {
		return nil, fmt.Errorf("jira view %s: %w", key, err)
	}
	return parseIssueDetail(out)
}

// IssueOrg returns the issue's org/scope custom field value, or "" when unset.
// `workitem view` is the ONLY acli command that returns custom fields (search
// rejects them, see searchFields) and it takes one key per call at roughly 5s,
// so callers must treat this as expensive and cache the result.
func (c *Client) IssueOrg(ctx context.Context, key string) (string, error) {
	out, err := c.runner.Run(ctx, []string{"jira", "workitem", "view"}, []string{key},
		[]string{"--fields", OrgScopeFieldID, "--json"})
	if err != nil {
		return "", fmt.Errorf("jira view %s org: %w", key, err)
	}
	return parseIssueOrg(out)
}

// parseIssueOrg pulls the org/scope value out of a `workitem view` payload.
func parseIssueOrg(raw string) (string, error) {
	var item acliIssue
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return "", fmt.Errorf("parsing org field: %w", err)
	}
	return item.field(OrgScopeFieldID), nil
}

// TransitionIssue moves a work item to the target status (by name). acli resolves
// the matching workflow transition; an invalid target for the issue's current
// status returns an error. --yes skips the interactive confirmation.
func (c *Client) TransitionIssue(ctx context.Context, key, status string) error {
	_, err := c.runner.Run(ctx, []string{"jira", "workitem", "transition"}, nil,
		[]string{"--key", key, "--status", status, "--yes"})
	if err != nil {
		return fmt.Errorf("jira transition %s -> %q: %w", key, status, err)
	}
	return nil
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
			Priority:  item.field("priority"),
			Created:   item.field("created"),
			Org:       item.field(OrgScopeFieldID),
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
		Priority:    item.field("priority"),
	}
	if rawParent := item.Fields["parent"]; rawParent != nil {
		var p struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(rawParent, &p) == nil {
			detail.ParentKey = p.Key
		}
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
