package tooling

import (
	"context"
	"fmt"
)

// maxContextHits caps how many contexts search_contexts returns to keep tool
// payloads small and focused.
const maxContextHits = 25

// maxBodyChars truncates long text bodies returned to the model to protect the
// context window.
const maxBodyChars = 8000

func strArg(args map[string]any, k string) string {
	if v, ok := args[k].(string); ok {
		return v
	}
	return ""
}

func truncate(s string) string {
	if len(s) <= maxBodyChars {
		return s
	}
	return s[:maxBodyChars] + "…[truncated]"
}

// ── Tool payloads ────────────────────────────────────────────────────────────

type contextHit struct {
	ContextID string `json:"contextId"`
	Key       string `json:"key"`
	Status    string `json:"status"`
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

func registerContextTools(r *Registry, d Deps) {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}

	r.Register(Tool{
		Name:        "search_contexts",
		Description: "Full-text search across ALL contexts and their content (Jira issues/comments, Slack threads, texts, files, branches). Returns matching contexts with their id and key. This is the same search the app's search bar uses.",
		Params: map[string]any{
			"type":       "object",
			"properties": map[string]any{"query": strProp("keywords to search for")},
			"required":   []string{"query"},
		},
		Handler: func(ctx context.Context, args map[string]any, tctx Ctx) (any, error) {
			query := strArg(args, "query")
			if query == "" {
				return nil, fmt.Errorf("query is required")
			}
			ids, err := d.Store.SearchContextIDs(ctx, query)
			if err != nil {
				return nil, err
			}
			hits := make([]contextHit, 0, len(ids))
			for _, id := range ids {
				if len(hits) >= maxContextHits {
					break
				}
				c, err := d.Store.GetContext(ctx, id)
				if err != nil {
					continue
				}
				if tctx.OrgID != "" && c.OrgID != tctx.OrgID {
					continue
				}
				hits = append(hits, contextHit{ContextID: c.ID, Key: c.Key, Status: c.Status})
			}
			return map[string]any{"count": len(hits), "contexts": hits}, nil
		},
	})

	r.Register(Tool{
		Name:        "get_context",
		Description: "List the elements inside one context: Jira issues (key/summary/status), Slack threads, text notes, uploaded files, and relations. Use item ids from here with get_item.",
		Params: map[string]any{
			"type":       "object",
			"properties": map[string]any{"contextId": strProp("the context id")},
			"required":   []string{"contextId"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			contextID := strArg(args, "contextId")
			if contextID == "" {
				return nil, fmt.Errorf("contextId is required")
			}
			de, err := d.Svc.GetContextDetail(ctx, contextID)
			if err != nil {
				return nil, err
			}
			issues := make([]issueBrief, 0, len(de.Issues))
			for _, is := range de.Issues {
				issues = append(issues, issueBrief{ID: is.ID, Key: is.JiraKey, Summary: is.Summary, Status: is.Status})
			}
			threads := make([]namedItem, 0, len(de.SlackThreads))
			for _, t := range de.SlackThreads {
				threads = append(threads, namedItem{ID: t.ID, Name: t.ChannelName})
			}
			texts := make([]namedItem, 0, len(de.ContextTexts))
			for _, t := range de.ContextTexts {
				texts = append(texts, namedItem{ID: t.ID, Name: t.Name})
			}
			files := make([]namedItem, 0, len(de.UploadedFiles))
			for _, f := range de.UploadedFiles {
				files = append(files, namedItem{ID: f.ID, Name: f.OriginalName})
			}
			relations := make([]relationBrief, 0, len(de.Relations))
			for _, rel := range de.Relations {
				relations = append(relations, relationBrief{TargetContextID: rel.TargetID, TargetKey: rel.TargetKey, RelationType: rel.RelationType})
			}
			return map[string]any{
				"contextId": de.Context.ID,
				"key":       de.Context.Key,
				"issues":    issues,
				"threads":   threads,
				"texts":     texts,
				"files":     files,
				"relations": relations,
			}, nil
		},
	})

	r.Register(Tool{
		Name:        "search_within_context",
		Description: "Find the elements inside a single context whose text matches a query. Returns matching item ids grouped by type.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"contextId": strProp("the context id"),
				"query":     strProp("keywords to search for"),
			},
			"required": []string{"contextId", "query"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			contextID, query := strArg(args, "contextId"), strArg(args, "query")
			if contextID == "" || query == "" {
				return nil, fmt.Errorf("contextId and query are required")
			}
			m, err := d.Store.SearchContextItems(ctx, contextID, query)
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
				"prIds":      m.PRIDs,
			}, nil
		},
	})

	r.Register(Tool{
		Name:        "get_item",
		Description: "Read the full text of one element. itemType is one of: issue, thread, text, file.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"contextId": strProp("the context id"),
				"itemType":  strProp("issue | thread | text | file"),
				"itemId":    strProp("the element id"),
			},
			"required": []string{"contextId", "itemType", "itemId"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			contextID, itemType, itemID := strArg(args, "contextId"), strArg(args, "itemType"), strArg(args, "itemId")
			if contextID == "" || itemType == "" || itemID == "" {
				return nil, fmt.Errorf("contextId, itemType and itemId are required")
			}
			de, err := d.Svc.GetContextDetail(ctx, contextID)
			if err != nil {
				return nil, err
			}
			switch itemType {
			case "issue":
				for _, is := range de.Issues {
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
				for _, t := range de.SlackThreads {
					if t.ID == itemID {
						return map[string]any{"type": "thread", "channel": t.ChannelName, "content": truncate(t.Content)}, nil
					}
				}
			case "text":
				for _, t := range de.ContextTexts {
					if t.ID == itemID {
						return map[string]any{"type": "text", "name": t.Name, "content": truncate(t.Content)}, nil
					}
				}
			case "file":
				for _, f := range de.UploadedFiles {
					if f.ID == itemID {
						return map[string]any{"type": "file", "name": f.OriginalName, "content": truncate(f.ExtractedText)}, nil
					}
				}
			default:
				return nil, fmt.Errorf("unknown itemType %q (use issue|thread|text|file)", itemType)
			}
			return nil, fmt.Errorf("%s %q not found in context", itemType, itemID)
		},
	})

	r.Register(Tool{
		Name:        "list_related_contexts",
		Description: "List contexts related to the given context (dependencies / links), with the relation type and target key.",
		Params: map[string]any{
			"type":       "object",
			"properties": map[string]any{"contextId": strProp("the context id")},
			"required":   []string{"contextId"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			contextID := strArg(args, "contextId")
			if contextID == "" {
				return nil, fmt.Errorf("contextId is required")
			}
			rels, err := d.Store.ListContextRelations(ctx, contextID)
			if err != nil {
				return nil, err
			}
			out := make([]relationBrief, 0, len(rels))
			for _, rel := range rels {
				out = append(out, relationBrief{TargetContextID: rel.TargetID, TargetKey: rel.TargetKey, RelationType: rel.RelationType})
			}
			return map[string]any{"count": len(out), "related": out}, nil
		},
	})
}
