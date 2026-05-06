package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newLogListCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "log-list",
		Short: "List context log entries",
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

			logs, err := env.contextLog.ListLogs(ctx, contextID)
			if err != nil {
				return fmt.Errorf("listing logs: %w", err)
			}
			return writeJSON(logs)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}

func newLogCreateCmd() *cobra.Command {
	var contextID, summaryMd string
	cmd := &cobra.Command{
		Use:   "log-create",
		Short: "Create a manual context log entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" || summaryMd == "" {
				return fmt.Errorf("--context-id and --summary-md are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			log, err := env.contextLog.CreateManualLog(ctx, contextID, summaryMd)
			if err != nil {
				return fmt.Errorf("creating log: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&summaryMd, "summary-md", "", "Markdown summary content")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("summary-md")
	return cmd
}

func newLogUpdateCmd() *cobra.Command {
	var id, summaryMd string
	cmd := &cobra.Command{
		Use:   "log-update",
		Short: "Update a context log entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" || summaryMd == "" {
				return fmt.Errorf("--id and --summary-md are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			log, err := env.contextLog.UpdateLog(ctx, id, summaryMd)
			if err != nil {
				return fmt.Errorf("updating log: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Log entry UUID")
	cmd.Flags().StringVar(&summaryMd, "summary-md", "", "Markdown summary content")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("summary-md")
	return cmd
}

func newLogDeleteCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "log-delete",
		Short: "Delete a context log entry",
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

			if err := env.contextLog.DeleteLog(ctx, id); err != nil {
				return fmt.Errorf("deleting log: %w", err)
			}
			return writeJSON(map[string]string{"deleted": id})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Log entry UUID")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newLogAttachFileCmd() *cobra.Command {
	var logID, fileID string
	cmd := &cobra.Command{
		Use:   "log-attach-file",
		Short: "Attach an uploaded file to a log entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck
			if err := env.contextLog.AttachFile(ctx, logID, fileID); err != nil {
				return fmt.Errorf("attaching file: %w", err)
			}
			return writeJSON(map[string]string{"logId": logID, "fileId": fileID})
		},
	}
	cmd.Flags().StringVar(&logID, "log-id", "", "Log entry UUID")
	cmd.Flags().StringVar(&fileID, "file-id", "", "Uploaded file UUID")
	_ = cmd.MarkFlagRequired("log-id")
	_ = cmd.MarkFlagRequired("file-id")
	return cmd
}

func newLogDetachFileCmd() *cobra.Command {
	var logID, fileID string
	cmd := &cobra.Command{
		Use:   "log-detach-file",
		Short: "Remove an uploaded file from a log entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck
			if err := env.contextLog.DetachFile(ctx, logID, fileID); err != nil {
				return fmt.Errorf("detaching file: %w", err)
			}
			return writeJSON(map[string]string{"logId": logID, "fileId": fileID})
		},
	}
	cmd.Flags().StringVar(&logID, "log-id", "", "Log entry UUID")
	cmd.Flags().StringVar(&fileID, "file-id", "", "Uploaded file UUID")
	_ = cmd.MarkFlagRequired("log-id")
	_ = cmd.MarkFlagRequired("file-id")
	return cmd
}

func newConsolidatedGetCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "consolidated-get",
		Short: "Get the consolidated context log",
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

			log, err := env.contextLog.GetConsolidated(ctx, contextID)
			if err != nil {
				return fmt.Errorf("getting consolidated: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}

func newConsolidatedUpdateCmd() *cobra.Command {
	var contextID, contentMd string
	cmd := &cobra.Command{
		Use:   "consolidated-update",
		Short: "Overwrite the consolidated log with user-provided markdown",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if contextID == "" || contentMd == "" {
				return fmt.Errorf("--context-id and --content-md are required")
			}
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			log, err := env.contextLog.UpdateConsolidatedManual(ctx, contextID, contentMd)
			if err != nil {
				return fmt.Errorf("updating consolidated: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contentMd, "content-md", "", "Markdown content")
	_ = cmd.MarkFlagRequired("context-id")
	_ = cmd.MarkFlagRequired("content-md")
	return cmd
}

func newConsolidatedInitCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "consolidated-init",
		Short: "AI-driven bootstrap of the consolidated log",
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

			snap, err := env.contextLog.Repo().SnapshotFull(ctx, contextID)
			if err != nil {
				return fmt.Errorf("snapshotting context: %w", err)
			}
			log, err := env.contextLog.Initialize(ctx, contextID, snap)
			if err != nil {
				return fmt.Errorf("initializing consolidated: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}

func newConsolidatedAnalyzeCmd() *cobra.Command {
	var contextID string
	cmd := &cobra.Command{
		Use:   "consolidated-analyze",
		Short: "On-demand AI change-log analysis",
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

			snap, err := env.contextLog.Repo().SnapshotFull(ctx, contextID)
			if err != nil {
				return fmt.Errorf("snapshotting context: %w", err)
			}
			log, err := env.contextLog.AnalyzeNow(ctx, contextID, snap)
			if err != nil {
				return fmt.Errorf("analyzing changes: %w", err)
			}
			return writeJSON(log)
		},
	}
	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	_ = cmd.MarkFlagRequired("context-id")
	return cmd
}
