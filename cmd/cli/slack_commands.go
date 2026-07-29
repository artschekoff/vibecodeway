package main

import (
	"context"
	"fmt"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	"github.com/spf13/cobra"
)

func newSlackListWorkspacesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "slack-list-workspaces",
		Short: "List available Slack workspaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			results, err := env.svc.ListSlackWorkspaces(ctx)
			if err != nil {
				return fmt.Errorf("listing workspaces: %w", err)
			}
			return writeJSON(results)
		},
	}
	return setAgentDocs(cmd,
		"List Slack workspaces the app is connected to.",
		"(none)",
		`["workspace-name",...]`,
		`context-consolidator-cli slack-list-workspaces`,
	)
}

func newSlackSearchChannelsCmd() *cobra.Command {
	var workspace, pattern string
	cmd := &cobra.Command{
		Use:   "slack-search-channels",
		Short: "Search Slack channels by name pattern",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if workspace == "" || pattern == "" {
				return fmt.Errorf("--workspace and --pattern are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			results, err := env.svc.SearchSlackChannels(ctx, workspace, pattern)
			if err != nil {
				return fmt.Errorf("searching channels: %w", err)
			}
			return writeJSON(results)
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Slack workspace name")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Channel name pattern, e.g. '*deploy*'")
	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("pattern")
	return setAgentDocs(cmd,
		"Search Slack channels by name pattern in a workspace. Use to find channel IDs before slack-add-channels.",
		"--workspace STRING (required)\n--pattern STRING (required)",
		`[{"channelId":"string","channelName":"string"}]`,
		`context-consolidator-cli slack-search-channels --workspace myworkspace --pattern "*deploy*"`,
	)
}

func newSlackAddChannelsCmd() *cobra.Command {
	var contextID, workspace, channelIDs, channelNames, startFrom string
	cmd := &cobra.Command{
		Use:   "slack-add-channels",
		Short: "Add Slack channels to a context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" || workspace == "" || channelIDs == "" || channelNames == "" {
				return fmt.Errorf("--context-id, --workspace, --channel-ids, and --channel-names are required")
			}
			ids := splitComma(channelIDs)
			names := splitComma(channelNames)
			if len(ids) != len(names) {
				return fmt.Errorf("--channel-ids and --channel-names must have the same number of elements")
			}

			channels := make([]slackstore.SlackChannelInput, len(ids))
			for i := range ids {
				channels[i] = slackstore.SlackChannelInput{
					ChannelID:   ids[i],
					ChannelName: names[i],
					StartFrom:   startFrom,
				}
			}

			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if err := env.svc.AddSlackChannels(ctx, contextID, workspace, channels); err != nil {
				return fmt.Errorf("adding channels: %w", err)
			}
			return writeJSON(map[string]int{"added": len(channels)})
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Slack workspace name")
	cmd.Flags().StringVar(&channelIDs, "channel-ids", "", "Comma-separated channel IDs")
	cmd.Flags().StringVar(&channelNames, "channel-names", "", "Comma-separated channel names")
	cmd.Flags().StringVar(&startFrom, "start-from", "", "Date filter (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("workspace")
	return setAgentDocs(cmd,
		"Add one or more Slack channels to a context. --channel-ids and --channel-names must be the same length.",
		"--context-id UUID (required)\n--workspace STRING (required)\n--channel-ids STRING (required; comma-separated)\n--channel-names STRING (required; comma-separated)\n--start-from YYYY-MM-DD (optional)",
		`{"added":"int"}`,
		`context-consolidator-cli slack-add-channels --context-id <uuid> --workspace myworkspace --channel-ids C123,C456 --channel-names general,deploy`,
	)
}

func newSlackListChannelsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "slack-list-channels",
		Short: "List Slack channels for a context",
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

			channels, err := env.svc.ListSlackChannels(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing channels: %w", err)
			}
			return writeJSON(channels)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return setAgentDocs(cmd,
		"List Slack channels linked to a context.",
		"--context-id UUID (required)",
		`[{"id":"uuid","channelId":"string","channelName":"string","workspace":"string","isActive":"bool"}]`,
		`context-consolidator-cli slack-list-channels --context-id <uuid>`,
	)
}

func newSlackRemoveChannelCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-remove-channel",
		Short: "Remove a Slack channel from a context",
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

			if err := env.svc.RemoveSlackChannel(ctx, id); err != nil {
				return fmt.Errorf("removing channel: %w", err)
			}
			return writeJSON(map[string]string{"removed": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Channel record UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Remove a Slack channel from a context. Use slack-list-channels to find the UUID.",
		"--id UUID (required; channel record UUID)",
		`{"removed":"uuid"}`,
		`context-consolidator-cli slack-remove-channel --id <uuid>`,
	)
}

func newSlackToggleChannelCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-toggle-channel",
		Short: "Toggle is_active for a Slack channel",
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

			ch, err := env.svc.ToggleSlackChannelActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling channel: %w", err)
			}
			return writeJSON(ch)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Channel record UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Toggle is_active for a Slack channel. Inactive channels are excluded from context loads.",
		"--id UUID (required)",
		`{"id":"uuid","channelId":"string","isActive":"bool",...}`,
		`context-consolidator-cli slack-toggle-channel --id <uuid>`,
	)
}

func newSlackLoadThreadCmd() *cobra.Command {
	var contextID, workspace, channelID, threadTS, channelName, permalink, searchQuery string
	cmd := &cobra.Command{
		Use:   "slack-load-thread",
		Short: "Load a Slack thread and store it in a context",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" || workspace == "" || channelID == "" || threadTS == "" {
				return fmt.Errorf("--context-id, --workspace, --channel-id, and --thread-ts are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if err := env.svc.LoadSlackContext(ctx, jiraservice.LoadSlackContextArgs{
				ContextID:   contextID,
				Workspace:   workspace,
				ChannelID:   channelID,
				ThreadTS:    threadTS,
				ChannelName: channelName,
				Permalink:   permalink,
				SearchQuery: searchQuery,
			}); err != nil {
				return fmt.Errorf("loading thread: %w", err)
			}
			return writeJSON(map[string]string{"loaded": threadTS})
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Slack workspace name")
	cmd.Flags().StringVar(&channelID, "channel-id", "", "Slack channel ID")
	cmd.Flags().StringVar(&threadTS, "thread-ts", "", "Thread timestamp")
	cmd.Flags().StringVar(&channelName, "channel-name", "", "Channel name (optional)")
	cmd.Flags().StringVar(&permalink, "permalink", "", "Permalink (optional)")
	cmd.Flags().StringVar(&searchQuery, "search-query", "", "Search query (optional)")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("workspace")
	_ = cmd.MarkFlagRequired("channel-id")
	_ = cmd.MarkFlagRequired("thread-ts")
	return setAgentDocs(cmd,
		"Fetch a Slack thread by channel ID and thread timestamp, then store it in a context.",
		"--context-id UUID (required)\n--workspace STRING (required)\n--channel-id STRING (required)\n--thread-ts STRING (required)\n--channel-name STRING (optional)\n--permalink STRING (optional)\n--search-query STRING (optional)",
		`{"loaded":"thread-ts"}`,
		`context-consolidator-cli slack-load-thread --context-id <uuid> --workspace myworkspace --channel-id C123ABC --thread-ts 1234567890.123456`,
	)
}

func newSlackListThreadsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "slack-list-threads",
		Short: "List Slack threads for a context",
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

			threads, err := env.svc.ListSlackThreads(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing threads: %w", err)
			}
			return writeJSON(threads)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return setAgentDocs(cmd,
		"List stored Slack threads for a context.",
		"--context-id UUID (required)",
		`[{"id":"uuid","channelId":"string","threadTs":"string","isActive":"bool","createdAt":"RFC3339"}]`,
		`context-consolidator-cli slack-list-threads --context-id <uuid>`,
	)
}

func newSlackDeleteThreadCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-delete-thread",
		Short: "Remove a Slack thread",
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

			if err := env.svc.DeleteSlackThread(ctx, id); err != nil {
				return fmt.Errorf("deleting thread: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Thread record UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Remove a Slack thread from local storage.",
		"--id UUID (required)",
		`{"deleted":"uuid"}`,
		`context-consolidator-cli slack-delete-thread --id <uuid>`,
	)
}

func newSlackToggleThreadCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-toggle-thread",
		Short: "Toggle is_active for a Slack thread",
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

			t, err := env.svc.ToggleSlackThreadActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling thread: %w", err)
			}
			return writeJSON(t)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Thread record UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Toggle is_active for a Slack thread. Inactive threads are excluded from context loads.",
		"--id UUID (required)",
		`{"id":"uuid","threadTs":"string","isActive":"bool",...}`,
		`context-consolidator-cli slack-toggle-thread --id <uuid>`,
	)
}

func newSlackListDMConfigsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "slack-list-dm-configs",
		Short: "List Slack DM configs for a context",
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

			configs, err := env.svc.ListSlackDMConfigs(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing dm configs: %w", err)
			}
			return writeJSON(configs)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return setAgentDocs(cmd,
		"List Slack DM configs for a context. Each config controls which DMs are fetched from a workspace.",
		"--context-id UUID (required)",
		`[{"id":"uuid","workspace":"string","startFrom":"string","isActive":"bool"}]`,
		`context-consolidator-cli slack-list-dm-configs --context-id <uuid>`,
	)
}

func newSlackUpsertDMConfigCmd() *cobra.Command {
	var contextID, workspace, startFrom string
	cmd := &cobra.Command{
		Use:   "slack-upsert-dm-config",
		Short: "Create or update a Slack DM config and load DM content",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" || workspace == "" {
				return fmt.Errorf("--context-id and --workspace are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			cfg, err := env.svc.UpsertSlackDMConfig(ctx, contextID, workspace, startFrom)
			if err != nil {
				return fmt.Errorf("upserting dm config: %w", err)
			}
			return writeJSON(cfg)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Slack workspace name")
	cmd.Flags().StringVar(&startFrom, "start-from", "", "Date filter (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("workspace")
	return setAgentDocs(cmd,
		"Create or update a Slack DM config for a workspace, then load DM content.",
		"--context-id UUID (required)\n--workspace STRING (required)\n--start-from YYYY-MM-DD (optional)",
		`{"id":"uuid","workspace":"string","startFrom":"string","isActive":"bool",...}`,
		`context-consolidator-cli slack-upsert-dm-config --context-id <uuid> --workspace myworkspace`,
	)
}

func newSlackDeleteDMConfigCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-delete-dm-config",
		Short: "Delete a Slack DM config",
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

			if err := env.svc.DeleteSlackDMConfig(ctx, id); err != nil {
				return fmt.Errorf("deleting dm config: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "DM config UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Delete a Slack DM config and its associated entries.",
		"--id UUID (required)",
		`{"deleted":"uuid"}`,
		`context-consolidator-cli slack-delete-dm-config --id <uuid>`,
	)
}

func newSlackToggleDMConfigCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-toggle-dm-config",
		Short: "Toggle is_active for a Slack DM config",
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

			cfg, err := env.svc.ToggleSlackDMConfigActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling dm config: %w", err)
			}
			return writeJSON(cfg)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "DM config UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Toggle is_active for a Slack DM config. Inactive configs are excluded from context loads.",
		"--id UUID (required)",
		`{"id":"uuid","workspace":"string","isActive":"bool",...}`,
		`context-consolidator-cli slack-toggle-dm-config --id <uuid>`,
	)
}

func newSlackRemoveDMEntryCmd() *cobra.Command {
	var dmConfigID, entryID string
	cmd := &cobra.Command{
		Use:   "slack-remove-dm-entry",
		Short: "Remove a DM entry from a DM config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dmConfigID == "" || entryID == "" {
				return fmt.Errorf("--dm-config-id and --entry-id are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if err := env.svc.RemoveSlackDMEntry(ctx, dmConfigID, entryID); err != nil {
				return fmt.Errorf("removing dm entry: %w", err)
			}
			return writeJSON(map[string]string{"removed": entryID})
		},
	}
	cmd.Flags().StringVar(&dmConfigID, "dm-config-id", "", "DM config UUID")
	cmd.Flags().StringVar(&entryID, "entry-id", "", "DM entry ID")
	_ = cmd.MarkFlagRequired("dm-config-id")
	_ = cmd.MarkFlagRequired("entry-id")
	return setAgentDocs(cmd,
		"Remove a specific DM entry from a DM config.",
		"--dm-config-id UUID (required)\n--entry-id STRING (required)",
		`{"removed":"entry-id"}`,
		`context-consolidator-cli slack-remove-dm-entry --dm-config-id <uuid> --entry-id <entry-uuid>`,
	)
}

func newSlackPreviewDMsCmd() *cobra.Command {
	var workspace, startFrom string
	cmd := &cobra.Command{
		Use:   "slack-preview-dms",
		Short: "Preview DMs without persisting",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if workspace == "" {
				return fmt.Errorf("--workspace is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			content, err := env.svc.PreviewSlackDMs(ctx, workspace, startFrom)
			if err != nil {
				return fmt.Errorf("previewing dms: %w", err)
			}
			return writeJSON(map[string]string{"content": content})
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "Slack workspace name")
	cmd.Flags().StringVar(&startFrom, "start-from", "", "Date filter (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("workspace")
	return setAgentDocs(cmd,
		"Fetch DMs from a Slack workspace without storing them. Useful before slack-upsert-dm-config.",
		"--workspace STRING (required)\n--start-from YYYY-MM-DD (optional)",
		`{"content":"string"}`,
		`context-consolidator-cli slack-preview-dms --workspace myworkspace`,
	)
}
