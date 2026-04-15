// Command context-consolidator-mcp is a Model Context Protocol server that exposes
// saved Jira contexts to AI clients (e.g. Cursor) via stdio JSON-RPC.
//
// Register in Cursor's mcp.json:
//
//	{
//	  "mcpServers": {
//	    "context-consolidator": {
//	      "type": "stdio",
//	      "command": "/path/to/bin/context-consolidator-mcp"
//	    }
//	  }
//	}
//
// Prerequisites: acli must be installed and authenticated via `acli jira auth login`.
package main

import (
	"fmt"
	"os"

	"github.com/artschekoff/context-consolidator/internal/jira"
	"github.com/artschekoff/context-consolidator/internal/jira/acli"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/slack"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	serverName    = "context-consolidator-mcp"
	serverVersion = "1.0.1"
)

func main() {
	root := &cobra.Command{
		Use:           "context-consolidator-mcp",
		Short:         "Jira Context MCP server for AI clients",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run()
		},
	}

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "context-consolidator-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger, err := buildLogger()
	if err != nil {
		return fmt.Errorf("initializing logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	s, err := appstore.Open(appstore.DefaultDBPath())
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	aRunner := acli.NewRunner(logger)
	jiraClient := jira.NewClient(aRunner, logger)
	slackRunner := slack.NewRunner(logger)

	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())

	jiraProvider := jiraservice.NewJiraProvider(jiraRepo, jiraClient, logger)
	slackProvider := jiraservice.NewSlackProvider(slackRepo, slackRunner, logger)
	textProvider := jiraservice.NewTextProvider(s, logger)

	svc := jiraservice.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, logger, jiraProvider, slackProvider, textProvider)

	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	registerTools(mcpServer, svc, logger)

	logger.Info("context-consolidator-mcp starting", zap.String("version", serverVersion))

	if err := server.ServeStdio(mcpServer); err != nil {
		return fmt.Errorf("serving stdio: %w", err)
	}
	return nil
}

func buildLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stderr"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	logger, err := cfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}
	return logger, nil
}
