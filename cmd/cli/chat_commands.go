package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newChatListCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "chat-list",
		Short: "List chat messages for a context",
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

			msgs, err := env.contextLog.ListChat(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing chat: %w", err)
			}
			return writeJSON(msgs)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}

func newChatSendCmd() *cobra.Command {
	var contextID, message string
	cmd := &cobra.Command{
		Use:   "chat-send",
		Short: "Send a chat message and get AI reply",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" || message == "" {
				return fmt.Errorf("--context-id and --message are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			msg, err := env.contextLog.ChatSend(ctx, contextID, message)
			if err != nil {
				return fmt.Errorf("sending chat: %w", err)
			}
			return writeJSON(msg)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&message, "message", "", "User message text")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func newChatApplySuggestionCmd() *cobra.Command {
	var messageID string
	cmd := &cobra.Command{
		Use:   "chat-apply-suggestion",
		Short: "Apply an AI-suggested consolidated log rewrite",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if messageID == "" {
				return fmt.Errorf("--message-id is required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			log, err := env.contextLog.ApplyChatSuggestion(ctx, messageID)
			if err != nil {
				return fmt.Errorf("applying suggestion: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&messageID, "message-id", "", "Chat message UUID")
	_ = cmd.MarkFlagRequired("message-id")
	return cmd
}
