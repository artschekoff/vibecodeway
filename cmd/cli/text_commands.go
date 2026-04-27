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
