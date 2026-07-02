package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newContextListCmd() *cobra.Command {
	var includeArchived bool
	cmd := &cobra.Command{
		Use:   "context-list",
		Short: "List contexts",
		Long: `List active contexts with their UUID, key, status, and item counts.

Output: JSON array of context objects.`,
		Example: `  context-consolidator-cli context-list
  context-consolidator-cli context-list --include-archived`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if includeArchived {
				list, err := env.svc.ListAllContexts(ctx)
				if err != nil {
					return fmt.Errorf("listing contexts: %w", err)
				}
				return writeJSON(list)
			}
			list, err := env.svc.ListContexts(ctx)
			if err != nil {
				return fmt.Errorf("listing contexts: %w", err)
			}
			return writeJSON(list)
		},
	}
	cmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include archived contexts")
	return cmd
}

func newContextCreateCmd() *cobra.Command {
	var key, orgID string
	cmd := &cobra.Command{
		Use:   "context-create",
		Short: "Create a new context",
		Long: `Create a new context with a unique key tied to an organization.

Required: --key, --org-id

Output: JSON context object (id, key, orgId, status, createdAt, updatedAt).`,
		Example: `  context-consolidator-cli context-create --key sprint-42 --org-id org-abc123`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			if orgID == "" {
				return fmt.Errorf("--org-id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			c, err := env.svc.CreateContext(ctx, key, orgID)
			if err != nil {
				return fmt.Errorf("creating context: %w", err)
			}
			return writeJSON(c)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "Context key, e.g. 'sprint-42'")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID the context belongs to")
	_ = cmd.MarkFlagRequired("key")
	_ = cmd.MarkFlagRequired("org-id")
	return cmd
}

func newContextUpdateCmd() *cobra.Command {
	var id, key, status string
	cmd := &cobra.Command{
		Use:   "context-update",
		Short: "Update a context",
		Long: `Update the key or status of a context.

Required: --id
Optional: --key (new key), --status (active|archived)

Output: JSON context object.`,
		Example: `  context-consolidator-cli context-update --id <uuid> --key sprint-43
  context-consolidator-cli context-update --id <uuid> --status archived`,
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

			c, err := env.svc.UpdateContext(ctx, id, key, status)
			if err != nil {
				return fmt.Errorf("updating context: %w", err)
			}
			return writeJSON(c)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Context UUID")
	cmd.Flags().StringVar(&key, "key", "", "New context key")
	cmd.Flags().StringVar(&status, "status", "", "New status (active|archived)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newContextDeleteCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "context-delete",
		Short: "Delete a context and all its data",
		Long: `Permanently delete a context and all its associated data (Jira issues, Slack
channels/threads/DMs, text entries, uploaded files). This cannot be undone.

Required: --id

Output: JSON {"deleted": "<uuid>"}.`,
		Example: `  context-consolidator-cli context-delete --id <uuid>`,
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

			if err := env.svc.DeleteContext(ctx, id); err != nil {
				return fmt.Errorf("deleting context: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newContextToggleStatusCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "context-toggle-status",
		Short: "Toggle a context between active and archived",
		Long: `Toggle a context between active and archived status.

Required: --id

Output: JSON context object with updated status.`,
		Example: `  context-consolidator-cli context-toggle-status --id <uuid>`,
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

			c, err := env.svc.ToggleContextStatus(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling status: %w", err)
			}
			return writeJSON(c)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newContextGetCmd() *cobra.Command {
	var contextID, contextKey string
	cmd := &cobra.Command{
		Use:   "context-get",
		Short: "Get full context detail",
		Long: `Get full context detail including Jira config, Slack config, and item counts.

Provide either --context-id or --context-key.

Output: JSON context detail object.`,
		Example: `  context-consolidator-cli context-get --context-key sprint-42
  context-consolidator-cli context-get --context-id <uuid>`,
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
			detail, err := env.svc.GetContextDetail(ctx, id)
			if err != nil {
				return fmt.Errorf("getting context: %w", err)
			}
			return writeJSON(detail)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Context key")
	return cmd
}

func newSetJiraConfigCmd() *cobra.Command {
	var contextID, searchQuery string
	cmd := &cobra.Command{
		Use:   "set-jira-config",
		Short: "Set the Jira JQL search query for a context",
		Long: `Set the JQL search query used by jira-search to discover new issues for a context.

Required: --context-id
Optional: --search-query (JQL string; omit to clear)

Output: JSON context object.`,
		Example: `  context-consolidator-cli set-jira-config --context-id <uuid> --search-query "project = JH AND sprint = 42"`,
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

			p, err := env.svc.SetJiraConfig(ctx, contextID, searchQuery)
			if err != nil {
				return fmt.Errorf("setting jira config: %w", err)
			}
			return writeJSON(p)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&searchQuery, "search-query", "", "JQL query")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}

func newSetSlackConfigCmd() *cobra.Command {
	var contextID, workspace, searchQuery string
	cmd := &cobra.Command{
		Use:   "set-slack-config",
		Short: "Set the Slack config for a context",
		Long: `Set the Slack workspace and default search query for a context.

Required: --context-id
Optional: --workspace (workspace name), --search-query (Slack search string)

Output: JSON context object.`,
		Example: `  context-consolidator-cli set-slack-config --context-id <uuid> --workspace myworkspace --search-query "in:general"`,
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

			p, err := env.svc.SetSlackConfig(ctx, contextID, workspace, searchQuery)
			if err != nil {
				return fmt.Errorf("setting slack config: %w", err)
			}
			return writeJSON(p)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Slack workspace name")
	cmd.Flags().StringVar(&searchQuery, "search-query", "", "Slack search query")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}
