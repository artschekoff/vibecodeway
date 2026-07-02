package main

import (
	"context"
	"fmt"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	"github.com/spf13/cobra"
)

func newSlackListWorkspacesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "slack-list-workspaces",
		Short: "List available Slack workspaces",
		Long: `List Slack workspaces the app is connected to.

Output: JSON array of workspace names.`,
		Example: `  context-consolidator-cli slack-list-workspaces`,
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
}

func newSlackSearchChannelsCmd() *cobra.Command {
	var workspace, pattern string
	cmd := &cobra.Command{
		Use:   "slack-search-channels",
		Short: "Search Slack channels by name pattern",
		Long: `Search Slack channels by name pattern in a workspace. Use this to find channel
IDs before calling slack-add-channels.

Required: --workspace, --pattern

Output: JSON array of {channelId, channelName}.`,
		Example: `  context-consolidator-cli slack-search-channels --workspace myworkspace --pattern "*deploy*"`,
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
	return cmd
}

func newSlackAddChannelsCmd() *cobra.Command {
	var contextID, workspace, channelIDs, channelNames, startFrom string
	cmd := &cobra.Command{
		Use:   "slack-add-channels",
		Short: "Add Slack channels to a context",
		Long: `Add one or more Slack channels to a context.

--channel-ids and --channel-names must be the same length and comma-separated.

Required: --context-id, --workspace, --channel-ids, --channel-names
Optional: --start-from (YYYY-MM-DD, only load messages from this date)

Output: JSON {"added": N}.`,
		Example: `  context-consolidator-cli slack-add-channels --context-id <uuid> --workspace myworkspace \
    --channel-ids C123,C456 --channel-names general,deploy
  context-consolidator-cli slack-add-channels --context-id <uuid> --workspace myworkspace \
    --channel-ids C123 --channel-names general --start-from 2024-01-01`,
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
	return cmd
}

func newSlackListChannelsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "slack-list-channels",
		Short: "List Slack channels for a context",
		Long: `List Slack channels linked to a context.

Required: --context-id

Output: JSON array of channel records (id, channelId, channelName, workspace, isActive).`,
		Example: `  context-consolidator-cli slack-list-channels --context-id <uuid>`,
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
	return cmd
}

func newSlackRemoveChannelCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-remove-channel",
		Short: "Remove a Slack channel from a context",
		Long: `Remove a Slack channel from a context. Use slack-list-channels to find the UUID.

Required: --id (channel record UUID)

Output: JSON {"removed": "<uuid>"}.`,
		Example: `  context-consolidator-cli slack-remove-channel --id <uuid>`,
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
	return cmd
}

func newSlackToggleChannelCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-toggle-channel",
		Short: "Toggle is_active for a Slack channel",
		Long: `Toggle is_active for a Slack channel. Inactive channels are excluded from context loads.

Required: --id

Output: JSON channel object with updated isActive.`,
		Example: `  context-consolidator-cli slack-toggle-channel --id <uuid>`,
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
	return cmd
}

func newSlackLoadThreadCmd() *cobra.Command {
	var contextID, workspace, channelID, threadTS, channelName, permalink, searchQuery string
	cmd := &cobra.Command{
		Use:   "slack-load-thread",
		Short: "Load a Slack thread and store it in a context",
		Long: `Fetch a Slack thread by channel ID and thread timestamp, then store it in a context.

Required: --context-id, --workspace, --channel-id, --thread-ts
Optional: --channel-name, --permalink, --search-query

Output: JSON {"loaded": "<thread-ts>"}.`,
		Example: `  context-consolidator-cli slack-load-thread \
    --context-id <uuid> --workspace myworkspace \
    --channel-id C123ABC --thread-ts 1234567890.123456`,
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
	return cmd
}

func newSlackListThreadsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "slack-list-threads",
		Short: "List Slack threads for a context",
		Long: `List stored Slack threads for a context.

Required: --context-id

Output: JSON array of thread records (id, channelId, threadTs, isActive, createdAt).`,
		Example: `  context-consolidator-cli slack-list-threads --context-id <uuid>`,
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
	return cmd
}

func newSlackDeleteThreadCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-delete-thread",
		Short: "Remove a Slack thread",
		Long: `Remove a Slack thread from local storage.

Required: --id

Output: JSON {"deleted": "<uuid>"}.`,
		Example: `  context-consolidator-cli slack-delete-thread --id <uuid>`,
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
	return cmd
}

func newSlackToggleThreadCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-toggle-thread",
		Short: "Toggle is_active for a Slack thread",
		Long: `Toggle is_active for a Slack thread. Inactive threads are excluded from context loads.

Required: --id

Output: JSON thread object with updated isActive.`,
		Example: `  context-consolidator-cli slack-toggle-thread --id <uuid>`,
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
	return cmd
}

func newSlackListDMConfigsCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "slack-list-dm-configs",
		Short: "List Slack DM configs for a context",
		Long: `List Slack DM configs for a context. Each config controls which DMs are fetched
from a given workspace.

Required: --context-id

Output: JSON array of DM config records (id, workspace, startFrom, isActive).`,
		Example: `  context-consolidator-cli slack-list-dm-configs --context-id <uuid>`,
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
	return cmd
}

func newSlackUpsertDMConfigCmd() *cobra.Command {
	var contextID, workspace, startFrom string
	cmd := &cobra.Command{
		Use:   "slack-upsert-dm-config",
		Short: "Create or update a Slack DM config and load DM content",
		Long: `Create or update a Slack DM config for a workspace, then load DM content.

Required: --context-id, --workspace
Optional: --start-from (YYYY-MM-DD, only load DMs from this date)

Output: JSON DM config record.`,
		Example: `  context-consolidator-cli slack-upsert-dm-config --context-id <uuid> --workspace myworkspace
  context-consolidator-cli slack-upsert-dm-config --context-id <uuid> --workspace myworkspace --start-from 2024-01-01`,
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
	return cmd
}

func newSlackDeleteDMConfigCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-delete-dm-config",
		Short: "Delete a Slack DM config",
		Long: `Delete a Slack DM config and its associated entries.

Required: --id

Output: JSON {"deleted": "<uuid>"}.`,
		Example: `  context-consolidator-cli slack-delete-dm-config --id <uuid>`,
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
	return cmd
}

func newSlackToggleDMConfigCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "slack-toggle-dm-config",
		Short: "Toggle is_active for a Slack DM config",
		Long: `Toggle is_active for a Slack DM config. Inactive configs are excluded from context loads.

Required: --id

Output: JSON DM config object with updated isActive.`,
		Example: `  context-consolidator-cli slack-toggle-dm-config --id <uuid>`,
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
	return cmd
}

func newSlackRemoveDMEntryCmd() *cobra.Command {
	var dmConfigID, entryID string
	cmd := &cobra.Command{
		Use:   "slack-remove-dm-entry",
		Short: "Remove a DM entry from a DM config",
		Long: `Remove a specific DM entry from a DM config.

Required: --dm-config-id, --entry-id

Output: JSON {"removed": "<entry-id>"}.`,
		Example: `  context-consolidator-cli slack-remove-dm-entry --dm-config-id <uuid> --entry-id <entry-uuid>`,
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
	return cmd
}

func newSlackPreviewDMsCmd() *cobra.Command {
	var workspace, startFrom string
	cmd := &cobra.Command{
		Use:   "slack-preview-dms",
		Short: "Preview DMs without persisting",
		Long: `Fetch DMs from a Slack workspace without storing them. Useful for previewing
content before calling slack-upsert-dm-config.

Required: --workspace
Optional: --start-from (YYYY-MM-DD)

Output: JSON {"content": "<formatted DM text>"}.`,
		Example: `  context-consolidator-cli slack-preview-dms --workspace myworkspace
  context-consolidator-cli slack-preview-dms --workspace myworkspace --start-from 2024-01-01`,
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
	return cmd
}
