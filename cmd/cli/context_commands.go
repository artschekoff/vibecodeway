package main

import (
	"context"
	"fmt"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/spf13/cobra"
)

func newContextListCmd() *cobra.Command {
	var includeArchived bool
	cmd := &cobra.Command{
		Use:   "context-list",
		Short: "List contexts",
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
	return setAgentDocs(
		cmd,
		"List contexts with UUID, key, status, and item counts. By default only active contexts are returned.",
		"--include-archived bool (optional; default false)",
		`[{"id":"uuid","key":"string","status":"active|archived","orgId":"string","issueCount":"int",...}]`,
		`vibecodeway-cli context-list`,
	)
}

func newContextCreateCmd() *cobra.Command {
	var key, orgID string
	cmd := &cobra.Command{
		Use:   "context-create",
		Short: "Create a new context",
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
	return setAgentDocs(
		cmd,
		"Create a new context with a unique key tied to an organization.",
		"--key STRING (required)\n--org-id STRING (required)",
		`{"id":"uuid","key":"string","orgId":"string","status":"active","createdAt":"RFC3339","updatedAt":"RFC3339"}`,
		`vibecodeway-cli context-create --key sprint-42 --org-id org-abc123`,
	)
}

func newContextUpdateCmd() *cobra.Command {
	var id, key, status string
	cmd := &cobra.Command{
		Use:   "context-update",
		Short: "Update a context",
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
	return setAgentDocs(
		cmd,
		"Update the key or status of a context.",
		"--id UUID (required)\n--key STRING (optional)\n--status active|archived (optional)",
		`{"id":"uuid","key":"string","status":"active|archived",...}`,
		`vibecodeway-cli context-update --id <uuid> --key sprint-43`,
	)
}

func newContextDeleteCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "context-delete",
		Short: "Delete a context and all its data",
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
	return setAgentDocs(
		cmd,
		"Permanently delete a context and all associated data (Jira, Slack, text, files). Cannot be undone.",
		"--id UUID (required)",
		`{"deleted":"uuid"}`,
		`vibecodeway-cli context-delete --id <uuid>`,
	)
}

func newContextToggleStatusCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "context-toggle-status",
		Short: "Toggle a context between active and archived",
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
	return setAgentDocs(
		cmd,
		"Toggle a context between active and archived status.",
		"--id UUID (required)",
		`{"id":"uuid","key":"string","status":"active|archived",...}`,
		`vibecodeway-cli context-toggle-status --id <uuid>`,
	)
}

func newContextGetCmd() *cobra.Command {
	var contextID, contextKey string
	cmd := &cobra.Command{
		Use:   "context-get",
		Short: "Get full context detail",
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
	return setAgentDocs(
		cmd,
		"Get full context detail including Jira config, Slack config, issues, and item counts.",
		"--context-id UUID | --context-key STRING (one required)",
		`{"id":"uuid","key":"string","jiraConfig":{...},"slackConfig":{...},"issues":[...],...}`,
		`vibecodeway-cli context-get --context-key sprint-42`,
	)
}

func newSetJiraConfigCmd() *cobra.Command {
	var contextID, searchQuery string
	cmd := &cobra.Command{
		Use:   "set-jira-config",
		Short: "Set the Jira JQL search query for a context",
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
	return setAgentDocs(
		cmd,
		"Set the JQL search query used by jira-search to discover new issues for a context. Omit --search-query to clear.",
		"--context-id UUID (required)\n--search-query STRING (optional JQL; omit to clear)",
		`{"id":"uuid","key":"string","jiraConfig":{"searchQuery":"string"},...}`,
		`vibecodeway-cli set-jira-config --context-id <uuid> --search-query "project = JH AND sprint = 42"`,
	)
}

func newSetSlackConfigCmd() *cobra.Command {
	var contextID, workspace, searchQuery string
	cmd := &cobra.Command{
		Use:   "set-slack-config",
		Short: "Set the Slack config for a context",
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
	return setAgentDocs(
		cmd,
		"Set the Slack workspace and default search query for a context.",
		"--context-id UUID (required)\n--workspace STRING (optional)\n--search-query STRING (optional)",
		`{"id":"uuid","key":"string","slackConfig":{"workspace":"string","searchQuery":"string"},...}`,
		`vibecodeway-cli set-slack-config --context-id <uuid> --workspace myworkspace --search-query "in:general"`,
	)
}

// newOrgListCmd lists organizations so a context can be created in the right one.
func newOrgListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org-list",
		Short: "List organizations",
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			orgs, err := env.store.ListOrganizations(context.Background())
			if err != nil {
				return fmt.Errorf("listing organizations: %w", err)
			}
			if orgs == nil {
				orgs = []appstore.Organization{}
			}
			return writeJSON(orgs)
		},
	}
	return setAgentDocs(
		cmd,
		"List organizations with their UUID and name. Read-only. Use to pick the --org-id required by context-create.",
		"(no flags)",
		`[{"id":"uuid","name":"string","sortOrder":"int","createdAt":"RFC3339","updatedAt":"RFC3339"}]`,
		`vibecodeway-cli org-list`,
	)
}
