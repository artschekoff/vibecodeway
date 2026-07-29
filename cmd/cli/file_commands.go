package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newFileListCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "file-list",
		Short: "List uploaded files for a context",
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

			files, err := env.svc.ListUploadedFiles(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing files: %w", err)
			}
			return writeJSON(files)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return setAgentDocs(cmd,
		"List uploaded document files stored for a context.",
		"--context-id UUID (required)",
		`[{"id":"uuid","name":"string","fileType":"string","isActive":"bool","createdAt":"RFC3339","updatedAt":"RFC3339"}]`,
		`context-consolidator-cli file-list --context-id <uuid>`,
	)
}

func newFileDeleteCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "file-delete",
		Short: "Delete an uploaded file",
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

			if err := env.svc.DeleteUploadedFile(ctx, id); err != nil {
				return fmt.Errorf("deleting file: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "File record UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Permanently delete an uploaded file from a context.",
		"--id UUID (required)",
		`{"deleted":"uuid"}`,
		`context-consolidator-cli file-delete --id <uuid>`,
	)
}

func newFileToggleCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "file-toggle",
		Short: "Toggle is_active for an uploaded file",
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

			f, err := env.svc.ToggleUploadedFileActive(ctx, id)
			if err != nil {
				return fmt.Errorf("toggling file: %w", err)
			}
			return writeJSON(f)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "File record UUID")
	_ = cmd.MarkFlagRequired("id")
	return setAgentDocs(cmd,
		"Toggle is_active for an uploaded file. Inactive files are excluded from context loads.",
		"--id UUID (required)",
		`{"id":"uuid","name":"string","fileType":"string","isActive":"bool",...}`,
		`context-consolidator-cli file-toggle --id <uuid>`,
	)
}
