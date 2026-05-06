// Command context-consolidator-cli is the CLI entry point for the context-consolidator.
// It exposes write operations (add-text, add-file) that can be called from the MCP
// server, the GUI, or directly from the terminal.
//
// Usage:
//
//	context-consolidator-cli add-text --context-key sprint-42 --file-path notes.md
//	context-consolidator-cli add-file --context-key sprint-42 --file-path spec.pdf
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "context-consolidator-cli",
		Short:         "CLI for context-consolidator write operations",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Text / file write (Phase A)
	root.AddCommand(newAddTextCmd())
	root.AddCommand(newAddFileCmd())

	// Context CRUD
	root.AddCommand(newContextListCmd())
	root.AddCommand(newContextCreateCmd())
	root.AddCommand(newContextUpdateCmd())
	root.AddCommand(newContextDeleteCmd())
	root.AddCommand(newContextToggleStatusCmd())
	root.AddCommand(newContextGetCmd())
	root.AddCommand(newSetJiraConfigCmd())
	root.AddCommand(newSetSlackConfigCmd())

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

	// Context logs & consolidated log
	root.AddCommand(newLogListCmd())
	root.AddCommand(newLogListAllCmd())
	root.AddCommand(newLogCreateCmd())
	root.AddCommand(newLogUpdateCmd())
	root.AddCommand(newLogDeleteCmd())
	root.AddCommand(newLogAttachFileCmd())
	root.AddCommand(newLogDetachFileCmd())
	root.AddCommand(newConsolidatedGetCmd())
	root.AddCommand(newConsolidatedUpdateCmd())
	root.AddCommand(newConsolidatedInitCmd())
	root.AddCommand(newConsolidatedAnalyzeCmd())

	// Chat
	root.AddCommand(newChatListCmd())
	root.AddCommand(newChatSendCmd())
	root.AddCommand(newChatApplySuggestionCmd())

	// Auth & AI settings
	root.AddCommand(newAuthStatusCmd())
	root.AddCommand(newGetAISettingsCmd())
	root.AddCommand(newSetAISettingsCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "context-consolidator-cli: %v\n", err)
		os.Exit(1)
	}
}
