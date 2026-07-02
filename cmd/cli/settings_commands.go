package main

import (
	"context"
	"fmt"

	"github.com/artschekoff/context-consolidator/internal/store"
	"github.com/spf13/cobra"
)

// aiSettingsResult is the JSON shape written to stdout by ai-settings-get.
// It includes the source of the active key so callers can distinguish env vs DB.
type aiSettingsResult struct {
	Provider        string `json:"provider"`
	HasKey          bool   `json:"hasKey"`
	HasOpenAIKey    bool   `json:"hasOpenAIKey"`
	HasAnthropicKey bool   `json:"hasAnthropicKey"`
	Model           string `json:"model"`
	BaseURL         string `json:"baseUrl"`
	Source          string `json:"source"`
	UpdatedAt       string `json:"updatedAt"`
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth-status",
		Short: "Return the current acli authentication status",
		Long: `Return the current Jira/Slack authentication status for the acli tool.

Output: JSON {"status": "<value>"} where value is one of: authenticated, unauthenticated, error.`,
		Example: `  context-consolidator-cli auth-status`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			status, err := env.svc.AuthStatus(ctx)
			if err != nil {
				return fmt.Errorf("auth status: %w", err)
			}
			return writeJSON(map[string]string{"status": status})
		},
	}
}

func newGetAISettingsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ai-settings-get",
		Short: "Get AI configuration metadata including source (env or db)",
		Long: `Get AI configuration metadata. The source field indicates whether the active API
key comes from an environment variable (env) or the database (db). Keys are never
included in the output — only whether they are set.

Output: JSON {provider, hasKey, hasOpenAIKey, hasAnthropicKey, model, baseUrl, source, updatedAt}.`,
		Example: `  context-consolidator-cli ai-settings-get`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			settings, src, err := env.openAI.DescribeSettings(ctx)
			if err != nil {
				return fmt.Errorf("getting ai settings: %w", err)
			}
			return writeJSON(aiSettingsResult{
				Provider:        settings.Provider,
				HasKey:          settings.HasKey,
				HasOpenAIKey:    settings.HasOpenAIKey,
				HasAnthropicKey: settings.HasAnthropicKey,
				Model:           settings.Model,
				BaseURL:         settings.BaseURL,
				Source:          string(src),
				UpdatedAt:       settings.UpdatedAt,
			})
		},
	}
}

func newSetAISettingsCmd() *cobra.Command {
	var provider, apiKey, anthropicKey, model, baseURL string
	cmd := &cobra.Command{
		Use:   "ai-settings-set",
		Short: "Set AI configuration (provider, keys, model, baseURL)",
		Long: `Set AI configuration. Keys are stored in plain text in the database.
Any flag left empty retains its current value.

Optional: --provider (openai|anthropic|local), --api-key, --anthropic-key, --model, --base-url

Output: JSON AI settings object (keys are redacted).`,
		Example: `  context-consolidator-cli ai-settings-set --provider openai --model gpt-4o
  context-consolidator-cli ai-settings-set --provider anthropic --anthropic-key sk-ant-...`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			if err := env.store.SetAISettings(ctx, store.SetAISettingsStoreOpts{
				Provider:     provider,
				OpenAIKey:    apiKey,
				AnthropicKey: anthropicKey,
				Model:        model,
				BaseURL:      baseURL,
			}); err != nil {
				return fmt.Errorf("setting ai settings: %w", err)
			}
			settings, err := env.store.GetAISettings(ctx)
			if err != nil {
				return fmt.Errorf("reading back ai settings: %w", err)
			}
			return writeJSON(settings)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "LLM provider: openai, anthropic, local")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "OpenAI API key")
	cmd.Flags().StringVar(&anthropicKey, "anthropic-key", "", "Anthropic API key")
	cmd.Flags().StringVar(&model, "model", "", "Model name, e.g. gpt-4o or claude-3-5-sonnet-20241022")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Custom base URL for local/proxy providers")
	return cmd
}
