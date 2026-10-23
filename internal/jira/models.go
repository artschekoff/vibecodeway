// Package jira provides a typed client for querying Jira via the acli binary.
package jira

import (
	"encoding/json"
	"regexp"
	"strings"
)

// SearchResult represents a single issue returned by jira workitem search.
type SearchResult struct {
	Key       string `json:"key"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	Assignee  string `json:"assignee"`
	IssueType string `json:"issuetype"`
	Priority  string `json:"priority"`
	Created   string `json:"created"`
	// Org is the ticket's scope/service (this instance's "organization"), read from
	// the configured scope custom field (Client.SetOrgField). Empty when unset.
	Org string `json:"org"`
}

// IssueDetail represents a full issue returned by jira workitem view.
type IssueDetail struct {
	Key         string       `json:"key"`
	Summary     string       `json:"summary"`
	Status      string       `json:"status"`
	Assignee    string       `json:"assignee"`
	IssueType   string       `json:"issuetype"`
	Description string       `json:"description"`
	Updated     string       `json:"updated"`
	ParentKey   string       `json:"parentKey,omitempty"`
	ParentType  string       `json:"parentType,omitempty"` // issue type of the parent (e.g. Epic)
	Priority    string       `json:"priority"`
	Subtasks    []SubtaskRef `json:"subtasks,omitempty"`
	Links       []IssueLink  `json:"links,omitempty"`
}

// SubtaskRef is a simplified child-issue reference returned in the parent's subtasks field.
type SubtaskRef struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

// IssueLink represents a single Jira issue link on an issue.
// Direction is "outward" when this issue is the source, "inward" when this issue is the target.
type IssueLink struct {
	TypeName  string `json:"typeName"`
	Direction string `json:"direction"` // "outward" | "inward"
	LinkedKey string `json:"linkedKey"`
	// Snapshot of the linked issue as embedded in the link payload.
	Summary   string `json:"summary,omitempty"`
	Status    string `json:"status,omitempty"`
	IssueType string `json:"issueType,omitempty"`
	Priority  string `json:"priority,omitempty"`
}

// Comment represents a comment returned by jira workitem comment list.
type Comment struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

// parseString extracts a string value from a raw JSON value that may be a plain
// string or a nested object with a "name" or "displayName" field.
func parseString(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"displayName", "name", "value"} {
		if v, ok := obj[key]; ok {
			var str string
			if err := json.Unmarshal(v, &str); err == nil {
				return str
			}
		}
	}
	return string(raw)
}

// rawIssue is used for flexible parsing of acli JSON output.
type rawIssue map[string]json.RawMessage

func (r rawIssue) str(key string) string {
	return parseString(r[key])
}

// adfNode represents a single node in Atlassian Document Format.
type adfNode struct {
	Type    string                     `json:"type"`
	Text    string                     `json:"text"`
	Content []adfNode                  `json:"content"`
	Attrs   map[string]json.RawMessage `json:"attrs"`
}

// extractADFText converts an ADF JSON node to plain text.
// Falls back gracefully for plain strings or unknown formats.
func extractADFText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var node adfNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	var sb strings.Builder
	writeADFNode(&sb, node)
	return strings.TrimSpace(sb.String())
}

func writeADFNode(sb *strings.Builder, node adfNode) {
	switch node.Type {
	case "text":
		sb.WriteString(node.Text)
		return
	case "hardBreak":
		sb.WriteString("\n")
		return
	case "mention":
		if v, ok := node.Attrs["text"]; ok {
			var t string
			if err := json.Unmarshal(v, &t); err == nil {
				sb.WriteString(t)
			}
		}
		return
	case "inlineCard":
		if v, ok := node.Attrs["url"]; ok {
			var u string
			if err := json.Unmarshal(v, &u); err == nil {
				sb.WriteString(u)
			}
		}
		return
	}
	for _, child := range node.Content {
		writeADFNode(sb, child)
	}
	switch node.Type {
	case "paragraph", "heading", "blockquote", "codeBlock", "rule":
		sb.WriteString("\n")
	case "listItem":
		sb.WriteString("\n")
	}
}

// githubRepoRE matches GitHub repository URLs and captures owner and repo name.
var githubRepoRE = regexp.MustCompile(`https://github\.com/([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)`)

// ExtractGitHubRepos parses text for GitHub repository URLs and returns unique "owner/repo" strings.
// Strips trailing ".git" suffixes and path segments beyond the repo name.
func ExtractGitHubRepos(text string) []string {
	seen := make(map[string]bool)
	var repos []string
	for _, m := range githubRepoRE.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSuffix(m[2], ".git")
		ownerRepo := m[1] + "/" + name
		if !seen[ownerRepo] {
			seen[ownerRepo] = true
			repos = append(repos, ownerRepo)
		}
	}
	return repos
}
