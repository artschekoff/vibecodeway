// Command vibecodeway-cli is the CLI entry point for the vibecodeway.
// It exposes read/write operations that can be called from the GUI or directly
// from the terminal (including by coding agents via `describe`).
//
// Usage:
//
//	vibecodeway-cli describe
//	vibecodeway-cli add-text --context-key sprint-42 --file-path notes.md
//	vibecodeway-cli add-file --context-key sprint-42 --file-path spec.pdf
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "vibecodeway-cli",
		Short:         "CLI for vibecodeway (JSON in/out for coding agents)",
		Long:          "vibecodeway-cli — manage local Jira/Slack/text contexts. Every command emits JSON on stdout. Run `describe` for the full machine-readable schema (description, input, output, flags).",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Keep shell-completion out of `describe` and help — no output schema.
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(newDescribeCmd())
	root.AddCommand(newAgentToolCmd())

	// Text / file write
	root.AddCommand(newAddTextCmd())
	root.AddCommand(newAddFileCmd())

	// Context CRUD
	root.AddCommand(newContextListCmd())
	root.AddCommand(newContextCreateCmd())
	root.AddCommand(newContextUpdateCmd())
	root.AddCommand(newContextDeleteCmd())
	root.AddCommand(newContextToggleStatusCmd())
	root.AddCommand(newContextGetCmd())
	root.AddCommand(newOrgListCmd())
	root.AddCommand(newSetJiraConfigCmd())
	root.AddCommand(newSetSlackConfigCmd())

	// Event log
	root.AddCommand(newEventAddCmd())

	// Text entries
	root.AddCommand(newTextListCmd())
	root.AddCommand(newTextDeleteCmd())
	root.AddCommand(newTextToggleCmd())

	// Uploaded files
	root.AddCommand(newFileListCmd())
	root.AddCommand(newFileDeleteCmd())
	root.AddCommand(newFileToggleCmd())

	// Jira operations
	root.AddCommand(newJiraSearchCmd())
	root.AddCommand(newJiraAddCmd())
	root.AddCommand(newJiraSyncCommentsCmd())
	root.AddCommand(newJiraSearchJQLCmd())
	root.AddCommand(newJiraRefreshCmd())
	root.AddCommand(newReloadAllCmd())
	root.AddCommand(newJiraDeleteIssueCmd())
	root.AddCommand(newJiraDeleteCommentCmd())
	root.AddCommand(newJiraToggleIssueCmd())
	root.AddCommand(newJiraToggleCommentCmd())

	// Slack operations
	root.AddCommand(newSlackListWorkspacesCmd())
	root.AddCommand(newSlackSearchChannelsCmd())
	root.AddCommand(newSlackAddChannelsCmd())
	root.AddCommand(newSlackListChannelsCmd())
	root.AddCommand(newSlackRemoveChannelCmd())
	root.AddCommand(newSlackToggleChannelCmd())
	root.AddCommand(newSlackLoadThreadCmd())
	root.AddCommand(newSlackListThreadsCmd())
	root.AddCommand(newSlackDeleteThreadCmd())
	root.AddCommand(newSlackToggleThreadCmd())
	root.AddCommand(newSlackListDMConfigsCmd())
	root.AddCommand(newSlackUpsertDMConfigCmd())
	root.AddCommand(newSlackDeleteDMConfigCmd())
	root.AddCommand(newSlackToggleDMConfigCmd())
	root.AddCommand(newSlackRemoveDMEntryCmd())
	root.AddCommand(newSlackPreviewDMsCmd())

	// Auth & AI settings
	root.AddCommand(newAuthStatusCmd())
	root.AddCommand(newGetAISettingsCmd())
	root.AddCommand(newSetAISettingsCmd())

	// VCS operations
	root.AddCommand(newRepoListCmd())
	root.AddCommand(newBranchAddCmd())

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "vibecodeway-cli: %v\n", err)
		os.Exit(1)
	}
}
