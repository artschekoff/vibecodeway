package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newJiraSearchCmd() *cobra.Command {
	var contextID, contextKey string
	cmd := &cobra.Command{
		Use:   "jira-search",
		Short: "Run context's saved JQL and return new vs existing issues",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			id, err := resolveContextID(ctx, env, contextID, contextKey)
			if err != nil {
				return err
			}
			diff, err := env.svc.SearchNewIssues(ctx, id)
			if err != nil {
				return fmt.Errorf("searching jira: %w", err)
			}
			return writeJSON(diff)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Context key")
	return setAgentDocs(cmd,
		"Run the context's saved JQL against Jira and return a diff of new vs existing issues. Call before jira-add to preview imports.",
		"--context-id UUID | --context-key STRING (one required)",
		`{"new":[{"key":"string","summary":"string","status":"string"}],"existing":[{"key":"string","summary":"string","status":"string"}]}`,
		`context-consolidator-cli jira-search --context-key sprint-42`,
	)
}

func newJiraAddCmd() *cobra.Command {
	var contextID, jiraKeys string
	cmd := &cobra.Command{
		Use:   "jira-add",
		Short: "Add Jira issues to a context by key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" {
				return fmt.Errorf("--context-id is required")
			}
			if jiraKeys == "" {
				return fmt.Errorf("--jira-keys is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			keys := splitComma(jiraKeys)
			added, err := env.svc.AddIssuesToContext(ctx, contextID, keys)
			if err != nil {
				return fmt.Errorf("adding issues: %w", err)
			}
			newComments, err := env.svc.SyncAllComments(ctx, contextID)
			if err != nil {
				return fmt.Errorf("syncing comments: %w", err)
			}
			return writeJSON(map[string]int{"added": added, "newComments": newComments})
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&jiraKeys, "jira-keys", "", "Comma-separated Jira keys, e.g. 'JH-1,JH-5'")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("jira-keys")
	return setAgentDocs(cmd,
		"Add one or more Jira issues to a context by key, then sync comments.",
		"--context-id UUID (required)\n--jira-keys STRING (required; comma-separated, e.g. JH-1,JH-5)",
		`{"added":"int","newComments":"int"}`,
		`context-consolidator-cli jira-add --context-id <uuid> --jira-keys JH-1,JH-5,JH-12`,
	)
}

func newJiraSyncCommentsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "jira-sync-comments",
		Short: "Sync all comments for all issues in a context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" {
				return fmt.Errorf("--context-id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			n, err := env.svc.SyncAllComments(ctx, contextID)
			if err != nil {
				return fmt.Errorf("syncing comments: %w", err)
			}
			return writeJSON(map[string]int{"newComments": n})
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return setAgentDocs(cmd,
		"Fetch new comments from Jira for all issues already stored in a context.",
		"--context-id UUID (required)",
		`{"newComments":"int"}`,
		`context-consolidator-cli jira-sync-comments --context-id <uuid>`,
	)
}

func newJiraSearchJQLCmd() *cobra.Command {
	var jql string
	cmd := &cobra.Command{
		Use:   "jira-search-jql",
		Short: "Run an arbitrary JQL query",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jql == "" {
				return fmt.Errorf("--jql is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			results, err := env.svc.SearchJQL(ctx, jql)
			if err != nil {
				return fmt.Errorf("searching jql: %w", err)
			}
			return writeJSON(results)
		},
	}
	cmd.Flags().StringVar(&jql, "jql", "", "JQL query string")
	_ = cmd.MarkFlagRequired("jql")
	return setAgentDocs(cmd,
		"Run an arbitrary JQL query against Jira and return matching issues. Does not modify any context data.",
		"--jql STRING (required)",
		`[{"key":"string","summary":"string","status":"string","assignee":"string",...}]`,
		`context-consolidator-cli jira-search-jql --jql "project = JH AND status = 'In Progress'"`,
	)
}

func newJiraRefreshCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "jira-refresh",
		Short: "Refresh all items in a context from Jira and Slack",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" {
				return fmt.Errorf("--context-id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			result, err := env.svc.RefreshContext(ctx, contextID)
			if err != nil {
				return fmt.Errorf("refreshing context: %w", err)
			}
			return writeJSON(result)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return setAgentDocs(cmd,
		"Re-fetch all Jira issues and Slack data for a context from their upstream sources.",
		"--context-id UUID (required)",
		`{"contextId":"uuid","jira":{"updated":"int","new":"int"},"slack":{...},...}`,
		`context-consolidator-cli jira-refresh --context-id <uuid>`,
	)
}

func newJiraDeleteIssueCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "jira-delete-issue",
		Short: "Remove a Jira issue from a context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if err := env.svc.DeleteJiraIssue(ctx, id); err != nil {
				return fmt.Errorf("deleting issue: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Issue internal UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Remove a Jira issue and its comments from local storage. Does not modify Jira. Use context-get to find the internal UUID for a Jira key.",
		"--id UUID (required; internal UUID, not JH-1)",
		`{"deleted":"uuid"}`,
		`context-consolidator-cli jira-delete-issue --id <uuid>`,
	)
}

func newJiraDeleteCommentCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "jira-delete-comment",
		Short: "Remove a Jira comment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if err := env.svc.DeleteJiraComment(ctx, id); err != nil {
				return fmt.Errorf("deleting comment: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Comment internal UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Remove a Jira comment from local storage. Does not modify Jira.",
		"--id UUID (required; internal UUID)",
		`{"deleted":"uuid"}`,
		`context-consolidator-cli jira-delete-comment --id <uuid>`,
	)
}

func newJiraToggleIssueCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "jira-toggle-issue",
		Short: "Toggle is_active for a Jira issue",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			issue, err := env.svc.ToggleIssueActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling issue: %w", err)
			}
			return writeJSON(issue)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Issue internal UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Toggle is_active for a Jira issue. Inactive issues are excluded from context loads.",
		"--id UUID (required)",
		`{"id":"uuid","key":"string","isActive":"bool",...}`,
		`context-consolidator-cli jira-toggle-issue --id <uuid>`,
	)
}

func newJiraToggleCommentCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "jira-toggle-comment",
		Short: "Toggle is_active for a Jira comment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			comment, err := env.svc.ToggleCommentActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling comment: %w", err)
			}
			return writeJSON(comment)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Comment internal UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Toggle is_active for a Jira comment. Inactive comments are excluded from context loads.",
		"--id UUID (required)",
		`{"id":"uuid","isActive":"bool",...}`,
		`context-consolidator-cli jira-toggle-comment --id <uuid>`,
	)
}

func newReloadAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reload-all",
		Short: "Re-fetch data for every active context from Jira and Slack",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			results, err := env.svc.ReloadAllContexts(ctx)
			if err != nil {
				return fmt.Errorf("reloading contexts: %w", err)
			}
			return writeJSON(results)
		},
	}
	return setAgentDocs(cmd,
		"Re-fetch data for every active context from Jira and Slack. Runs a refresh for each context sequentially.",
		"(none)",
		`[{"contextId":"uuid","jira":{...},"slack":{...},...}]`,
		`context-consolidator-cli reload-all`,
	)
}

// splitComma splits a comma-separated string and trims each element.
func splitComma(s string) []string {
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
