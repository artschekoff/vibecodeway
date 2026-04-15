// Package jira provides a typed client for querying Jira via the acli binary.
package jira

import (
	"encoding/json"
	"strings"
)

// SearchResult represents a single issue returned by jira workitem search.
type SearchResult struct {
	Key       string `json:"key"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	Assignee  string `json:"assignee"`
	IssueType string `json:"issuetype"`
}

// IssueDetail represents a full issue returned by jira workitem view.
type IssueDetail struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	Assignee    string `json:"assignee"`
	IssueType   string `json:"issuetype"`
	Description string `json:"description"`
	Updated     string `json:"updated"`
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
