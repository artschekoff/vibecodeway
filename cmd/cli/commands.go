package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/artschekoff/context-consolidator/internal/extract"
	"github.com/artschekoff/context-consolidator/internal/jira"
	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	openaiclient "github.com/artschekoff/context-consolidator/internal/openai"
	"github.com/artschekoff/context-consolidator/internal/slack"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// passphraseEnvVar is read by the CLI to configure Slack and OpenAI operations.
// The GUI sets this env var when spawning CLI subprocesses.
const passphraseEnvVar = "CONTEXT_CONSOLIDATOR_PASSPHRASE"

// serviceEnv holds all objects needed to call ContextService and contextlog methods.
type serviceEnv struct {
	store      *appstore.Store
	svc        *jiraservice.ContextService
	contextLog *contextlog.Service
	openAI     *openaiclient.Client
	logger     *zap.Logger
}

// openServiceEnv opens the database, reads the passphrase from the env, and
// constructs ContextService + contextlog.Service (with optional AI).
func openServiceEnv() (*serviceEnv, error) {
	logger, err := zap.NewProduction(zap.WithCaller(false))
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	s, err := appstore.Open(appstore.DefaultDBPath())
	if err != nil {
		logger.Sync() //nolint:errcheck
		return nil, fmt.Errorf("opening database: %w", err)
	}

	passphrase := os.Getenv(passphraseEnvVar)

	aRunner := acli.NewRunner(logger)
	jiraClient := jira.NewClient(aRunner, logger)

	var slackOpts []slack.Option
	if passphrase != "" {
		slackOpts = append(slackOpts, slack.WithPassphrase(passphrase))
	}
	slackRunner := slack.NewRunner(logger, slackOpts...)

	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())

	jiraProvider := jiraservice.NewJiraProvider(jiraRepo, jiraClient, logger)
	slackProvider := jiraservice.NewSlackProvider(slackRepo, slackRunner, logger)
	textProvider := jiraservice.NewTextProvider(s, logger)

	svc := jiraservice.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, logger, jiraProvider, slackProvider, textProvider)

	openAI := openaiclient.NewClient(s, slackRunner.Passphrase, logger)
	contextLogRepo := contextlog.NewRepo(s.DB())
	contextLogSvc := contextlog.NewService(contextLogRepo, openAI.AsContextLogClient(), logger)
	svc.SetContextLog(contextLogSvc)

	return &serviceEnv{
		store:      s,
		svc:        svc,
		contextLog: contextLogSvc,
		openAI:     openAI,
		logger:     logger,
	}, nil
}

// resolveContextID returns the context UUID from --context-id or --context-key flags.
func resolveContextID(ctx context.Context, env *serviceEnv, contextID, contextKey string) (string, error) {
	if contextID != "" {
		return contextID, nil
	}
	if contextKey == "" {
		return "", fmt.Errorf("provide either --context-id or --context-key")
	}
	c, err := env.store.GetContextByKey(ctx, contextKey)
	if err != nil {
		return "", fmt.Errorf("context with key %q not found: %w", contextKey, err)
	}
	return c.ID, nil
}

// writeJSON serialises v as JSON to stdout. Exits 1 on marshal error.
func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// newAddTextCmd returns the add-text subcommand.
func newAddTextCmd() *cobra.Command {
	var (
		contextID  string
		contextKey string
		name       string
		filePath   string
		content    string
	)

	cmd := &cobra.Command{
		Use:   "add-text",
		Short: "Add a plain-text entry to a context",
		Long: `Add a plain-text entry to a context.

Provide either --file-path (reads the file from disk) or --content (inline text).
For .md files, raw Markdown is stored as-is. Output is JSON.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" && content == "" {
				return fmt.Errorf("provide either --file-path or --content")
			}

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

			if filePath != "" {
				data, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("reading file: %w", err)
				}
				content = string(data)
				if name == "" {
					name = filepath.Base(filePath)
				}
			}

			ct, err := env.svc.UpsertContextText(ctx, "", id, name, content)
			if err != nil {
				return fmt.Errorf("adding text: %w", err)
			}

			return writeJSON(ct)
		},
	}

	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Context key, e.g. 'sprint-42'")
	cmd.Flags().StringVar(&name, "name", "", "Display name for the text entry (defaults to filename when --file-path is used)")
	cmd.Flags().StringVar(&filePath, "file-path", "", "Path to a file on disk (.md, .txt, etc.)")
	cmd.Flags().StringVar(&content, "content", "", "Inline text content")

	return cmd
}

// newAddFileCmd returns the add-file subcommand.
func newAddFileCmd() *cobra.Command {
	var (
		contextID  string
		contextKey string
		filePath   string
	)

	cmd := &cobra.Command{
		Use:   "add-file",
		Short: "Upload a document file to a context",
		Long: `Upload a document file to a context.

The file is read from disk, text is extracted, and both the original binary and
extracted text are stored. Supports: pdf, docx, rtf, odt, txt, md. Output is JSON.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filePath == "" {
				return fmt.Errorf("--file-path is required")
			}

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

			info, err := os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("stat file: %w", err)
			}

			fileBytes, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			ext := strings.ToLower(filepath.Ext(filePath))
			var extractedText, fileType string

			switch ext {
			case ".md", ".markdown":
				fileType = "md"
				extractedText = strings.TrimSpace(string(fileBytes))
			default:
				extractedText, fileType, err = extract.FromFile(filePath)
				if err != nil {
					return fmt.Errorf("extracting text: %w", err)
				}
			}

			f, err := env.svc.InsertUploadedFile(ctx, id, filepath.Base(filePath), fileType, extractedText, fileBytes, info.Size())
			if err != nil {
				return fmt.Errorf("inserting file: %w", err)
			}

			return writeJSON(f)
		},
	}

	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Context key, e.g. 'sprint-42'")
	cmd.Flags().StringVar(&filePath, "file-path", "", "Absolute path to the file on disk")
	_ = cmd.MarkFlagRequired("file-path")

	return cmd
}
