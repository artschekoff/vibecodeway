package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cliclient "github.com/artschekoff/context-consolidator/internal/cli"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

func registerTools(s *server.MCPServer, store *appstore.Store, svc *jiraservice.ContextService, vcsStore *vcsstore.Store, cliRunner *cliclient.Runner, logger *zap.Logger) {
	h := &handlers{store: store, svc: svc, vcsStore: vcsStore, cli: cliRunner, logger: logger}

	s.AddTool(
		mcp.NewTool("load_context",
			mcp.WithDescription("Load all saved content for a context formatted as readable text: Jira issues (with comments), Slack channels/threads/DMs, plain-text entries, uploaded file contents, and linked repositories with PRs. Use the filter parameter to request only one source type. Use start_at to limit Jira/Slack items by date. Use skip_repos=true to omit the repositories section. By default only active (non-deactivated) items are returned."),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Context key (human-readable name), e.g. 'sprint-42'."),
			),
			mcp.WithString("filter",
				mcp.Description("Which sources to include: 'jira', 'slack', 'text' (plain texts + uploaded files), or omit for all sources."),
			),
			mcp.WithString("start_at",
				mcp.Description("ISO 8601 date (e.g. '2024-01-15' or '2024-01-15T09:00:00Z'). Only items updated or created on or after this date are included."),
			),
			mcp.WithString("skip_repos",
				mcp.Description("Set to 'true' to omit the repositories and pull-requests section from the output. Default: false (repos are included)."),
			),
			mcp.WithString("include_deactivated",
				mcp.Description("Set to 'true' to include items that have been switched off (deactivated) by the user. Default: false (deactivated items are excluded)."),
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
			mcp.WithDescription("Add selected Jira issues to a context by their Jira keys, then sync all comments for the entire context. For each added issue, subtasks, linked issues, and child issues (via parent JQL) are also added automatically. Call after context_search once the user has confirmed which issues to add."),
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

	s.AddTool(
		mcp.NewTool("context_add_text",
			mcp.WithDescription("Add a plain-text entry to a context. Provide either file_path (reads file from disk, supports .md) or content (inline text). Returns the created record as JSON."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key, e.g. 'sprint-42'. Use either context_id or context_key."),
			),
			mcp.WithString("name",
				mcp.Description("Display name for the text entry. Defaults to the filename when file_path is provided."),
			),
			mcp.WithString("file_path",
				mcp.Description("Absolute path to a file on disk (.md, .txt, etc.). Content is read and stored as plain text."),
			),
			mcp.WithString("content",
				mcp.Description("Inline text content. Use instead of file_path when text is already available."),
			),
		),
		h.handleContextAddText,
	)

	s.AddTool(
		mcp.NewTool("context_add_file",
			mcp.WithDescription("Upload a document file to a context. The file is read from disk, text is extracted, and both the original binary and extracted text are stored. Supports: pdf, docx, rtf, odt, txt, md."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key, e.g. 'sprint-42'. Use either context_id or context_key."),
			),
			mcp.WithString("file_path",
				mcp.Required(),
				mcp.Description("Absolute path to the file on disk."),
			),
		),
		h.handleContextAddFile,
	)

	s.AddTool(
		mcp.NewTool("context_log",
			mcp.WithDescription("Return the event log for a context as a JSON array of events, ordered by timestamp ascending."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key. Use either context_id or context_key."),
			),
		),
		h.handleContextLog,
	)

	s.AddTool(
		mcp.NewTool("context_log_add",
			mcp.WithDescription("Append an agent-authored message to the context log. Use this to record notes, summaries, or status updates about work performed by an AI agent. The message is stored as a new log entry and merged into the consolidated context log."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key (e.g. 'sprint-42'). Use either context_id or context_key."),
			),
			mcp.WithString("message",
				mcp.Required(),
				mcp.Description("The log message to append. Markdown is supported."),
			),
		),
		h.handleContextLogAdd,
	)

	s.AddTool(
		mcp.NewTool("context_set_status",
			mcp.WithDescription("Set the AI-proposed workflow status of a context. Use this to explicitly signal where a context stands in the workflow. Optionally include a comment explaining the situation, a recommended next step, and a log message that will be appended to the context log."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key, e.g. 'sprint-42'. Use either context_id or context_key."),
			),
			mcp.WithString("status",
				mcp.Required(),
				mcp.Description("Workflow status to set. Must be one of: not_started, in_progress, scoping, needs_attention, ready."),
			),
			mcp.WithString("comment",
				mcp.Description("Short explanation of the current situation (optional)."),
			),
			mcp.WithString("next_step",
				mcp.Description("Recommended next action, ideally under 20 words (optional)."),
			),
			mcp.WithString("message",
				mcp.Description("Log message to append to the context log alongside the status change (optional). Markdown supported."),
			),
		),
		h.handleContextSetStatus,
	)

	s.AddTool(
		mcp.NewTool("pr_list",
			mcp.WithDescription("List pull requests stored for a context. Returns PR status, review status, author, title, and URL."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key, e.g. 'sprint-42'. Use either context_id or context_key."),
			),
		),
		h.handlePRList,
	)

	s.AddTool(
		mcp.NewTool("pr_comments",
			mcp.WithDescription("Get PR comments for a context. By default returns all unresolved comments. Pass include_resolved='true' to get all comments including resolved ones."),
			mcp.WithString("context_id",
				mcp.Description("Context UUID. Use either context_id or context_key."),
			),
			mcp.WithString("context_key",
				mcp.Description("Context user-assigned key. Use either context_id or context_key."),
			),
			mcp.WithString("pr_number",
				mcp.Description("Optional. If provided, returns comments for that specific PR number only."),
			),
			mcp.WithString("include_resolved",
				mcp.Description("Set to 'true' to include resolved comments. Defaults to 'false' (unresolved only)."),
			),
		),
		h.handlePRComments,
	)

	s.AddTool(
		mcp.NewTool("pr_review_requests",
			mcp.WithDescription("List pull requests where the authenticated GitHub user's review has been requested. Returns PRs across all tracked repositories."),
		),
		h.handlePRReviewRequests,
	)
}

type handlers struct {
	store    *appstore.Store
	svc      *jiraservice.ContextService
	vcsStore *vcsstore.Store
	cli      *cliclient.Runner
	logger   *zap.Logger
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

	// Auto-add GitHub repos detected in issue descriptions
	reposAdded := 0
	if h.vcsStore != nil {
		repos := h.svc.ExtractGitHubReposFromIssues(ctx, contextID, keys)
		for _, ownerRepo := range repos {
			parts := strings.SplitN(ownerRepo, "/", 2)
			if len(parts) != 2 {
				continue
			}
			repo, err := h.vcsStore.Repos.EnsureRepo(ctx, "github", parts[0], parts[1])
			if err != nil {
				h.logger.Warn("auto-add repo: ensure repo failed", zap.String("repo", ownerRepo), zap.Error(err))
				continue
			}
			if _, err := h.vcsStore.Branches.AddBranch(ctx, contextID, repo.ID, "", ""); err != nil {
				h.logger.Warn("auto-add repo: add branch failed", zap.String("repo", ownerRepo), zap.Error(err))
				continue
			}
			reposAdded++
		}
	}

	msg := fmt.Sprintf("Added %d issues to context. Synced %d new comments.", added, newComments)
	if reposAdded > 0 {
		msg += fmt.Sprintf(" Auto-linked %d GitHub repositories from issue descriptions.", reposAdded)
	}
	return mcp.NewToolResultText(msg), nil
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

	texts := make([]appstore.ContextText, 0, len(detail.ContextTexts))
	for _, t := range detail.ContextTexts {
		if t.IsActive {
			texts = append(texts, t)
		}
	}
	filtered.ContextTexts = texts

	dmConfigs := make([]slackstore.SlackDMConfig, 0, len(detail.SlackDMConfigs))
	for _, dm := range detail.SlackDMConfigs {
		if dm.IsActive {
			dmConfigs = append(dmConfigs, dm)
		}
	}
	filtered.SlackDMConfigs = dmConfigs

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

func (h *handlers) handleContextLog(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	events, err := h.store.ListEvents(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing events: %v", err)), nil
	}
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (h *handlers) handleContextLogAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	message := strings.TrimSpace(req.GetString("message", ""))
	if message == "" {
		return mcp.NewToolResultError("message is required"), nil
	}

	stored, err := h.store.AddEvent(ctx, appstore.Event{
		ContextID:  id,
		Type:       appstore.EventTypeUserNote,
		SourceType: appstore.EventSourceUser,
		Summary:    message,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("adding event: %v", err)), nil
	}

	if err := h.store.MarkSnapshotStale(ctx, id); err != nil {
		h.logger.Warn("context_log_add: mark snapshot stale failed", zap.String("contextID", id), zap.Error(err))
	}

	return mcp.NewToolResultText(fmt.Sprintf("Log entry created (id=%s).", stored.ID)), nil
}

func (h *handlers) handleContextSetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	status := strings.TrimSpace(req.GetString("status", ""))
	if status == "" {
		return mcp.NewToolResultError("status is required"), nil
	}
	valid, err := h.svc.IsValidWorkflowStatus(ctx, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validating status: %v", err)), nil
	}
	if !valid {
		return mcp.NewToolResultError(fmt.Sprintf("invalid status %q — check available statuses in Settings", status)), nil
	}

	comment := strings.TrimSpace(req.GetString("comment", ""))
	nextStep := strings.TrimSpace(req.GetString("next_step", ""))
	message := strings.TrimSpace(req.GetString("message", ""))

	// Build the summary for the status-update event from all provided fields.
	parts := []string{"Status: " + status}
	if comment != "" {
		parts = append(parts, "Comment: "+comment)
	}
	if nextStep != "" {
		parts = append(parts, "Next step: "+nextStep)
	}
	if message != "" {
		parts = append(parts, message)
	}
	summary := strings.Join(parts, "\n")

	if _, err := h.store.AddEvent(ctx, appstore.Event{
		ContextID:  id,
		Type:       appstore.EventTypeStatusUpdate,
		SourceType: appstore.EventSourceUser,
		Summary:    summary,
	}); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("setting status: %v", err)), nil
	}

	if err := h.store.MarkSnapshotStale(ctx, id); err != nil {
		h.logger.Warn("context_set_status: mark snapshot stale failed", zap.String("contextID", id), zap.Error(err))
	}

	result := fmt.Sprintf("Status set to %q.", status)
	if message != "" {
		result += " Log entry appended."
	}
	return mcp.NewToolResultText(result), nil
}

func (h *handlers) handleLoadContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	filter := strings.ToLower(strings.TrimSpace(req.GetString("filter", "")))
	if filter != "" && filter != "jira" && filter != "slack" && filter != "text" {
		return mcp.NewToolResultError("filter must be 'jira', 'slack', 'text', or omitted"), nil
	}

	skipRepos := strings.ToLower(strings.TrimSpace(req.GetString("skip_repos", ""))) == "true"
	includeDeactivated := strings.EqualFold(strings.TrimSpace(req.GetString("include_deactivated", "")), "true")
	activeOnly := !includeDeactivated

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

	if err := h.svc.FormatContext(ctx, &buf, detail.Context.ID, filter, startAt, activeOnly); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Repos section — omitted when skip_repos=true or a specific filter is active
	if !skipRepos && filter == "" && h.vcsStore != nil {
		branches, err := h.vcsStore.Branches.ListBranches(ctx, detail.Context.ID)
		if err == nil && len(branches) > 0 {
			fmt.Fprintf(&buf, "\n--- REPOSITORIES (%d) ---\n\n", len(branches))
			for _, b := range branches {
				if b.Repo == nil {
					continue
				}
				platform := b.Repo.Platform
				repoFull := fmt.Sprintf("%s/%s", b.Repo.Owner, b.Repo.Name)
				labelPart := ""
				if b.Label != "" {
					labelPart = fmt.Sprintf(" [label: %s]", b.Label)
				}
				branchPart := " (tracking: all branches)"
				if b.Branch != "" {
					branchPart = fmt.Sprintf(" (branch: %s)", b.Branch)
				}
				fmt.Fprintf(&buf, "- %s:%s%s%s\n", platform, repoFull, labelPart, branchPart)
			}
			buf.WriteString("\n")
		}
	}

	if filter == "" && len(detail.Relations) > 0 {
		fmt.Fprintf(&buf, "\n--- RELATED CONTEXTS (%d) ---\n\n", len(detail.Relations))
		for _, r := range detail.Relations {
			fmt.Fprintf(&buf, "- %s → %s\n", r.RelationType, r.TargetKey)
		}
		buf.WriteString("\n")
	}

	// Logs section — always last, only when no filter is active.
	if filter == "" {
		events, err := h.store.ListEvents(ctx, detail.Context.ID)
		if err == nil && len(events) > 0 {
			fmt.Fprintf(&buf, "\n--- EVENTS (%d %s) ---\n\n", len(events), pluralWord(len(events), "entry", "entries"))
			for _, e := range events {
				fmt.Fprintf(&buf, "[%s · %s · %s]\n", jiraservice.FormatDateTime(e.Timestamp), strings.ToUpper(e.Type), strings.ToUpper(e.SourceType))
				fmt.Fprint(&buf, e.Summary)
				fmt.Fprint(&buf, "\n\n")
			}
		}
	}

	return mcp.NewToolResultText(buf.String()), nil
}

func pluralWord(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}

func (h *handlers) handleContextAddText(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath := strings.TrimSpace(req.GetString("file_path", ""))
	content := req.GetString("content", "")

	if filePath == "" && content == "" {
		return mcp.NewToolResultError("provide either file_path or content"), nil
	}

	contextID := req.GetString("context_id", "")
	contextKey := req.GetString("context_key", "")
	if contextID == "" && contextKey == "" {
		return mcp.NewToolResultError("provide either context_id or context_key"), nil
	}

	ct, err := h.cli.AddText(ctx, cliclient.AddTextOpts{
		ContextID:  contextID,
		ContextKey: contextKey,
		Name:       strings.TrimSpace(req.GetString("name", "")),
		FilePath:   filePath,
		Content:    content,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("adding text: %v", err)), nil
	}

	data, err := json.MarshalIndent(ct, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (h *handlers) handleContextAddFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath := strings.TrimSpace(req.GetString("file_path", ""))
	if filePath == "" {
		return mcp.NewToolResultError("file_path is required"), nil
	}

	contextID := req.GetString("context_id", "")
	contextKey := req.GetString("context_key", "")
	if contextID == "" && contextKey == "" {
		return mcp.NewToolResultError("provide either context_id or context_key"), nil
	}

	f, err := h.cli.AddFile(ctx, cliclient.AddFileOpts{
		ContextID:  contextID,
		ContextKey: contextKey,
		FilePath:   filePath,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("adding file: %v", err)), nil
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (h *handlers) handlePRList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.vcsStore == nil {
		return mcp.NewToolResultError("VCS store not configured"), nil
	}
	contextID, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	prs, err := h.vcsStore.PullRequests.ListPRsForContext(ctx, contextID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing PRs: %v", err)), nil
	}

	// Enrich with repo/branch info by joining through branches.
	branches, _ := h.vcsStore.Branches.ListBranches(ctx, contextID)
	branchByID := make(map[string]vcsstore.ContextVCSBranch, len(branches))
	for _, b := range branches {
		branchByID[b.ID] = b
	}

	type prOut struct {
		PRNumber     int    `json:"pr_number"`
		Title        string `json:"title"`
		Author       string `json:"author"`
		State        string `json:"state"`
		ReviewStatus string `json:"review_status"`
		URL          string `json:"url"`
		Platform     string `json:"platform,omitempty"`
		Repo         string `json:"repo,omitempty"`
		Branch       string `json:"branch,omitempty"`
	}
	out := make([]prOut, 0, len(prs))
	for _, pr := range prs {
		row := prOut{
			PRNumber:     pr.PRNumber,
			Title:        pr.Title,
			Author:       pr.Author,
			State:        pr.State,
			ReviewStatus: pr.ReviewStatus,
			URL:          pr.URL,
		}
		if b, ok := branchByID[pr.BranchID]; ok {
			row.Branch = b.Branch
			if b.Repo != nil {
				row.Platform = b.Repo.Platform
				row.Repo = b.Repo.FullName()
			}
		}
		out = append(out, row)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func (h *handlers) handlePRComments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.vcsStore == nil {
		return mcp.NewToolResultError("VCS store not configured"), nil
	}
	contextID, err := h.resolveContextID(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	includeResolved := strings.EqualFold(strings.TrimSpace(req.GetString("include_resolved", "")), "true")
	prNumberStr := strings.TrimSpace(req.GetString("pr_number", ""))

	prs, err := h.vcsStore.PullRequests.ListPRsForContext(ctx, contextID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing PRs: %v", err)), nil
	}

	var buf strings.Builder
	for _, pr := range prs {
		if prNumberStr != "" && fmt.Sprintf("%d", pr.PRNumber) != prNumberStr {
			continue
		}
		comments, err := h.vcsStore.PullRequests.ListCommentsForPRFiltered(ctx, pr.ID, includeResolved)
		if err != nil {
			h.logger.Warn("mcp pr_comments: load comments failed", zap.String("prID", pr.ID), zap.Error(err))
			continue
		}
		if len(comments) == 0 {
			continue
		}
		tree := vcsstore.BuildCommentTree(comments)
		fmt.Fprintf(&buf, "## PR #%d: %s\n", pr.PRNumber, pr.Title)
		fmt.Fprintf(&buf, "Author: %s | State: %s | Review: %s\n\n", pr.Author, pr.State, pr.ReviewStatus)
		writeMCPCommentTree(&buf, tree, "")
		buf.WriteString("\n")
	}

	if buf.Len() == 0 {
		return mcp.NewToolResultText("No comments found."), nil
	}
	return mcp.NewToolResultText(buf.String()), nil
}

func writeMCPCommentTree(w *strings.Builder, comments []vcsstore.PRComment, indent string) {
	for _, c := range comments {
		author := c.Author
		if author == "" {
			author = "Unknown"
		}
		resolved := ""
		if c.IsResolved {
			resolved = " [resolved]"
		}
		location := ""
		if c.Path != nil && *c.Path != "" {
			location = fmt.Sprintf(" [%s", *c.Path)
			if c.Line != nil {
				location += fmt.Sprintf(":%d", *c.Line)
			}
			location += "]"
		}
		fmt.Fprintf(w, "%s**%s**%s%s:\n", indent, author, location, resolved)
		for _, line := range strings.Split(c.Body, "\n") {
			fmt.Fprintf(w, "%s  %s\n", indent, line)
		}
		if len(c.Replies) > 0 {
			writeMCPCommentTree(w, c.Replies, indent+"  ")
		}
		w.WriteString("\n")
	}
}

func (h *handlers) handlePRReviewRequests(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.vcsStore == nil {
		return mcp.NewToolResultError("VCS store not configured"), nil
	}

	rrs, err := h.vcsStore.ReviewRequests.ListReviewRequests(ctx, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing review requests: %v", err)), nil
	}

	// Enrich with repo info.
	repoCache := make(map[string]*vcsstore.VCSRepo)
	type rrOut struct {
		PRNumber     int    `json:"pr_number"`
		PRTitle      string `json:"pr_title"`
		PRAuthor     string `json:"pr_author"`
		PRURL        string `json:"pr_url"`
		ReviewStatus string `json:"review_status"`
		Repo         string `json:"repo,omitempty"`
		Platform     string `json:"platform,omitempty"`
	}
	out := make([]rrOut, 0, len(rrs))
	for _, rr := range rrs {
		row := rrOut{
			PRNumber:     rr.PRNumber,
			PRTitle:      rr.PRTitle,
			PRAuthor:     rr.PRAuthor,
			PRURL:        rr.PRURL,
			ReviewStatus: rr.ReviewStatus,
		}
		if repo, ok := repoCache[rr.RepoID]; ok {
			if repo != nil {
				row.Repo = repo.FullName()
				row.Platform = repo.Platform
			}
		} else {
			repo, err := h.vcsStore.Repos.GetRepo(ctx, rr.RepoID)
			if err == nil {
				repoCache[rr.RepoID] = repo
				row.Repo = repo.FullName()
				row.Platform = repo.Platform
			} else {
				repoCache[rr.RepoID] = nil
			}
		}
		out = append(out, row)
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serializing: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
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

