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
	return cmd
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
	return cmd
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
	return cmd
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
	return cmd
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
	return cmd
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
	return cmd
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
	return cmd
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
	return cmd
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
	return cmd
}

func newReloadAllCmd() *cobra.Command {
	return &cobra.Command{
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
