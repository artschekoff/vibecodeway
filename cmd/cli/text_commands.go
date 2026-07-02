package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newTextListCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "text-list",
		Short: "List text entries for a context",
		Long: `List plain-text entries stored for a context.

Required: --context-id

Output: JSON array of text entry objects (id, name, content, isActive, createdAt, updatedAt).`,
		Example: `  context-consolidator-cli text-list --context-id <uuid>`,
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

			texts, err := env.svc.ListContextTexts(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing texts: %w", err)
			}
			return writeJSON(texts)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}

func newTextDeleteCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "text-delete",
		Short: "Delete a text entry",
		Long: `Permanently delete a plain-text entry from a context.

Required: --id

Output: JSON {"deleted": "<uuid>"}.`,
		Example: `  context-consolidator-cli text-delete --id <uuid>`,
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

			if err := env.svc.DeleteContextText(ctx, id); err != nil {
				return fmt.Errorf("deleting text: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Text entry UUID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newTextToggleCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "text-toggle",
		Short: "Toggle is_active for a text entry",
		Long: `Toggle is_active for a text entry. Inactive entries are excluded from context loads.

Required: --id

Output: JSON text entry object with updated isActive.`,
		Example: `  context-consolidator-cli text-toggle --id <uuid>`,
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

			t, err := env.svc.ToggleContextTextActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling text: %w", err)
			}
			return writeJSON(t)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Text entry UUID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}
