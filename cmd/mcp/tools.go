package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

func registerTools(s *server.MCPServer, svc *jiraservice.ContextService, logger *zap.Logger) {
	h := &handlers{svc: svc, logger: logger}

	s.AddTool(
		mcp.NewTool("load_context",
			mcp.WithDescription("Load all saved Jira issues (with comments) and Slack thread content for a context, formatted as readable text for analysis. Use the filter parameter to request only jira or only slack data. Use start_at to receive only items updated or created after that date."),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Context key (human-readable name), e.g. 'sprint-42'."),
			),
			mcp.WithString("filter",
				mcp.Description("Which sources to include: 'jira', 'slack', or omit for both."),
			),
			mcp.WithString("start_at",
				mcp.Description("ISO 8601 date (e.g. '2024-01-15' or '2024-01-15T09:00:00Z'). Only items updated or created on or after this date are included."),
			),
		),
		h.handleLoadContext,
	)

	s.AddTool(
		mcp.NewTool("context_list",
			mcp.WithDescription("List saved contexts with their key, status, and issue count. By default only active contexts are returned. Call this first to discover available contexts."),
			mcp.WithString("include_archived",
				mcp.Description("Set to 'true' to include archived (inactive) contexts in the results. Defaults to active-only."),
			),
		),
		h.handleContextList,
	)

	s.AddTool(
		mcp.NewTool("context_search",
			mcp.WithDescription("Run the context's saved JQL search against Jira and return a diff showing which issues are new (not yet in context) and which already exist. Call this before context_add to let the user decide what to include."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key (e.g. 'sprint-42'). Use either context_id or context_key."),
			),
		),
		h.handleContextSearch,
	)

	s.AddTool(
		mcp.NewTool("context_add",
			mcp.WithDescription("Add selected Jira issues to a context by their Jira keys, then sync all comments for the entire context. Call after context_search once the user has confirmed which issues to add."),
			mcp.WithString("context_id",
				mcp.Required(),
				mcp.Description("Context UUID."),
			),
			mcp.WithString("jira_keys",
				mcp.Required(),
				mcp.Description("Comma-separated Jira issue keys to add, e.g. 'JH-1,JH-5'."),
			),
		),
		h.handleContextAdd,
	)

	s.AddTool(
		mcp.NewTool("context_get",
			mcp.WithDescription("Get the full contents of a context: issues, comments, Slack channels/threads, and text files. By default only active items are returned. Use include_inactive='true' to also retrieve items that have been deactivated."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key. Use either context_id or context_key."),
			),
			mcp.WithString("include_inactive",
				mcp.Description("Set to 'true' to include inactive (deactivated) items in the results. Defaults to active-only."),
			),
		),
		h.handleContextGet,
	)
}

type handlers struct {
	svc    *jiraservice.ContextService
	logger *zap.Logger
}

func (h *handlers) handleContextList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeArchived := strings.EqualFold(strings.TrimSpace(req.GetString("include_archived", "")), "true")

	var (
		contexts []appstore.Context
		err      error
	)
	if includeArchived {
		contexts, err = h.svc.ListAllContexts(ctx)
	} else {
		contexts, err = h.svc.ListContexts(ctx)
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing contexts: %v", err)), nil
	}
	data, err := json.MarshalIndent(contexts, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (h *handlers) handleContextSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	diff, err := h.svc.SearchNewIssues(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("searching jira: %v", err)), nil
	}

	data, err := json.MarshalIndent(diff, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (h *handlers) handleContextAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	contextID := req.GetString("context_id", "")
	if contextID == "" {
		return mcp.NewToolResultError("context_id is required"), nil
	}

	rawKeys := req.GetString("jira_keys", "")
	if rawKeys == "" {
		return mcp.NewToolResultError("jira_keys is required"), nil
	}

	keys := splitAndTrim(rawKeys)
	added, err := h.svc.AddIssuesToContext(ctx, contextID, keys)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("adding issues: %v", err)), nil
	}

	newComments, err := h.svc.SyncAllComments(ctx, contextID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("syncing comments: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf(
		"Added %d issues to context. Synced %d new comments.",
		added, newComments,
	)), nil
}

func (h *handlers) handleContextGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	detail, err := h.svc.GetContextDetail(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("loading context: %v", err)), nil
	}

	includeInactive := strings.EqualFold(strings.TrimSpace(req.GetString("include_inactive", "")), "true")
	if !includeInactive {
		detail = filterActiveItems(detail)
	}

	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// filterActiveItems returns a shallow copy of detail containing only active issues,
// comments, Slack channels, Slack threads, and text files.
func filterActiveItems(detail *jiraservice.ContextDetail) *jiraservice.ContextDetail {
	filtered := *detail

	issues := make([]jirastore.Issue, 0, len(detail.Issues))
	for _, iss := range detail.Issues {
		if !iss.IsActive {
			continue
		}
		activeComments := make([]jirastore.Comment, 0, len(iss.Comments))
		for _, c := range iss.Comments {
			if c.IsActive {
				activeComments = append(activeComments, c)
			}
		}
		iss.Comments = activeComments
		issues = append(issues, iss)
	}
	filtered.Issues = issues

	channels := make([]slackstore.SlackChannel, 0, len(detail.SlackChannels))
	for _, ch := range detail.SlackChannels {
		if ch.IsActive {
			channels = append(channels, ch)
		}
	}
	filtered.SlackChannels = channels

	threads := make([]slackstore.SlackThread, 0, len(detail.SlackThreads))
	for _, t := range detail.SlackThreads {
		if t.IsActive {
			threads = append(threads, t)
		}
	}
	filtered.SlackThreads = threads

	files := make([]appstore.ContextFile, 0, len(detail.ContextFiles))
	for _, f := range detail.ContextFiles {
		if f.IsActive {
			files = append(files, f)
		}
	}
	filtered.ContextFiles = files

	return &filtered
}

// resolveContextID returns the context UUID from either context_id or context_key parameter.
func (h *handlers) resolveContextID(ctx context.Context, req mcp.CallToolRequest) (string, error) {
	if id := req.GetString("context_id", ""); id != "" {
		return id, nil
	}
	key := req.GetString("context_key", "")
	if key == "" {
		return "", fmt.Errorf("provide either context_id or context_key")
	}
	detail, err := h.svc.GetContextDetailByKey(ctx, key)
	if err != nil {
		return "", fmt.Errorf("context with key %q not found: %w", key, err)
	}
	return detail.Context.ID, nil
}

func (h *handlers) handleLoadContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	filter := strings.ToLower(strings.TrimSpace(req.GetString("filter", "")))
	if filter != "" && filter != "jira" && filter != "slack" {
		return mcp.NewToolResultError("filter must be 'jira', 'slack', or omitted"), nil
	}

	var startAt time.Time
	if raw := strings.TrimSpace(req.GetString("start_at", "")); raw != "" {
		startAt = jiraservice.ParseTime(raw)
		if startAt.IsZero() {
			return mcp.NewToolResultError(fmt.Sprintf("invalid start_at %q — use ISO 8601, e.g. '2024-01-15'", raw)), nil
		}
	}

	detail, err := h.svc.GetContextDetailByKey(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("context %q not found: %v", name, err)), nil
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "=== Context: %s (%s) ===\n", detail.Context.Key, detail.Context.Status)
	if detail.Jira != nil && detail.Jira.SearchQuery != "" {
		fmt.Fprintf(&buf, "JQL: %s\n", detail.Jira.SearchQuery)
	}
	if !startAt.IsZero() {
		fmt.Fprintf(&buf, "Filtered since: %s\n", startAt.Format("2006-01-02"))
	}
	buf.WriteString("\n")

	if err := h.svc.FormatContext(ctx, &buf, detail.Context.ID, filter, startAt); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(buf.String()), nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
