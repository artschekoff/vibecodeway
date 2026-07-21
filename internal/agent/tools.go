package agent

import (
	"context"
	"fmt"

	"github.com/artschekoff/context-consolidator/internal/openai"
)

// maxContextHits caps how many contexts search_contexts returns to keep tool
// payloads small and focused.
const maxContextHits = 25

// ToolInfo is a user-facing description of a tool the agent can call.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolCatalogue returns the tools available to the agent, for display in the UI.
func (a *Agent) ToolCatalogue() []ToolInfo {
	defs := a.toolDefs()
	out := make([]ToolInfo, 0, len(defs))
	for _, d := range defs {
		out = append(out, ToolInfo{Name: d.Name, Description: d.Description})
	}
	return out
}

// toolDefs is the tool catalogue advertised to the model. JSON-schema arguments
// keep the model's calls well-formed.
func (a *Agent) toolDefs() []openai.ToolDef {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	return []openai.ToolDef{
		{
			Name:        "search_contexts",
			Description: "Full-text search across ALL contexts and their content (Jira issues/comments, Slack threads, texts, files, branches). Returns matching contexts with their id and key.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"query": strProp("keywords to search for")},
				"required":   []string{"query"},
			},
		},
		{
			Name:        "get_context",
			Description: "List the elements inside one context: Jira issues (key/summary/status), Slack threads, text notes, uploaded files, and relations. Use item ids from here with get_item.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"contextId": strProp("the context id")},
				"required":   []string{"contextId"},
			},
		},
		{
			Name:        "search_within_context",
			Description: "Find the elements inside a single context whose text matches a query. Returns matching item ids grouped by type.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"contextId": strProp("the context id"),
					"query":     strProp("keywords to search for"),
				},
				"required": []string{"contextId", "query"},
			},
		},
		{
			Name:        "get_item",
			Description: "Read the full text of one element. itemType is one of: issue, thread, text, file.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"contextId": strProp("the context id"),
					"itemType":  strProp("issue | thread | text | file"),
					"itemId":    strProp("the element id"),
				},
				"required": []string{"contextId", "itemType", "itemId"},
			},
		},
		{
			Name:        "list_related_contexts",
			Description: "List contexts related to the given context (dependencies / links), with the relation type and target key.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"contextId": strProp("the context id")},
				"required":   []string{"contextId"},
			},
		},
	}
}

// ── Tool payloads ────────────────────────────────────────────────────────────

type contextHit struct {
	ContextID string `json:"contextId"`
	Key       string `json:"key"`
	Status    string `json:"status"`
}

func (a *Agent) toolSearchContexts(ctx context.Context, query, orgID string) (any, error) {
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	ids, err := a.store.SearchContextIDs(ctx, query)
	if err != nil {
		return nil, err
	}
	hits := make([]contextHit, 0, len(ids))
	for _, id := range ids {
		if len(hits) >= maxContextHits {
			break
		}
		c, err := a.store.GetContext(ctx, id)
		if err != nil {
			continue
		}
		if orgID != "" && c.OrgID != orgID {
			continue
		}
		hits = append(hits, contextHit{ContextID: c.ID, Key: c.Key, Status: c.Status})
	}
	return map[string]any{"count": len(hits), "contexts": hits}, nil
}

type issueBrief struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}
type namedItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type relationBrief struct {
	TargetContextID string `json:"targetContextId"`
	TargetKey       string `json:"targetKey"`
	RelationType    string `json:"relationType"`
}

func (a *Agent) toolGetContext(ctx context.Context, contextID string) (any, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextId is required")
	}
	d, err := a.svc.GetContextDetail(ctx, contextID)
	if err != nil {
		return nil, err
	}
	issues := make([]issueBrief, 0, len(d.Issues))
	for _, is := range d.Issues {
		issues = append(issues, issueBrief{ID: is.ID, Key: is.JiraKey, Summary: is.Summary, Status: is.Status})
	}
	threads := make([]namedItem, 0, len(d.SlackThreads))
	for _, t := range d.SlackThreads {
		threads = append(threads, namedItem{ID: t.ID, Name: t.ChannelName})
	}
	texts := make([]namedItem, 0, len(d.ContextTexts))
	for _, t := range d.ContextTexts {
		texts = append(texts, namedItem{ID: t.ID, Name: t.Name})
	}
	files := make([]namedItem, 0, len(d.UploadedFiles))
	for _, f := range d.UploadedFiles {
		files = append(files, namedItem{ID: f.ID, Name: f.OriginalName})
	}
	relations := make([]relationBrief, 0, len(d.Relations))
	for _, r := range d.Relations {
		relations = append(relations, relationBrief{TargetContextID: r.TargetID, TargetKey: r.TargetKey, RelationType: r.RelationType})
	}
	return map[string]any{
		"contextId": d.Context.ID,
		"key":       d.Context.Key,
		"issues":    issues,
		"threads":   threads,
		"texts":     texts,
		"files":     files,
		"relations": relations,
	}, nil
}

func (a *Agent) toolSearchWithinContext(ctx context.Context, contextID, query string) (any, error) {
	if contextID == "" || query == "" {
		return nil, fmt.Errorf("contextId and query are required")
	}
	m, err := a.store.SearchContextItems(ctx, contextID, query)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"issueIds":   m.IssueIDs,
		"threadIds":  m.ThreadIDs,
		"textIds":    m.TextIDs,
		"fileIds":    m.FileIDs,
		"channelIds": m.ChannelIDs,
		"branchIds":  m.BranchIDs,
	}, nil
}

func (a *Agent) toolGetItem(ctx context.Context, contextID, itemType, itemID string) (any, error) {
	if contextID == "" || itemType == "" || itemID == "" {
		return nil, fmt.Errorf("contextId, itemType and itemId are required")
	}
	d, err := a.svc.GetContextDetail(ctx, contextID)
	if err != nil {
		return nil, err
	}
	switch itemType {
	case "issue":
		for _, is := range d.Issues {
			if is.ID == itemID {
				comments := make([]map[string]string, 0, len(is.Comments))
				for _, cm := range is.Comments {
					comments = append(comments, map[string]string{"author": cm.Author, "body": truncate(cm.Body)})
				}
				return map[string]any{
					"type": "issue", "key": is.JiraKey, "summary": is.Summary,
					"status": is.Status, "assignee": is.Assignee, "issueType": is.IssueType,
					"description": truncate(is.Description), "comments": comments,
				}, nil
			}
		}
	case "thread":
		for _, t := range d.SlackThreads {
			if t.ID == itemID {
				return map[string]any{"type": "thread", "channel": t.ChannelName, "content": truncate(t.Content)}, nil
			}
		}
	case "text":
		for _, t := range d.ContextTexts {
			if t.ID == itemID {
				return map[string]any{"type": "text", "name": t.Name, "content": truncate(t.Content)}, nil
			}
		}
	case "file":
		for _, f := range d.UploadedFiles {
			if f.ID == itemID {
				return map[string]any{"type": "file", "name": f.OriginalName, "content": truncate(f.ExtractedText)}, nil
			}
		}
	default:
		return nil, fmt.Errorf("unknown itemType %q (use issue|thread|text|file)", itemType)
	}
	return nil, fmt.Errorf("%s %q not found in context", itemType, itemID)
}

func (a *Agent) toolListRelated(ctx context.Context, contextID string) (any, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextId is required")
	}
	rels, err := a.store.ListContextRelations(ctx, contextID)
	if err != nil {
		return nil, err
	}
	out := make([]relationBrief, 0, len(rels))
	for _, r := range rels {
		out = append(out, relationBrief{TargetContextID: r.TargetID, TargetKey: r.TargetKey, RelationType: r.RelationType})
	}
	return map[string]any{"count": len(out), "related": out}, nil
}
