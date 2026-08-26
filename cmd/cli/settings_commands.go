package main

import (
	"context"
	"fmt"

	"github.com/artschekoff/vibecodeway/internal/store"
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
	cmd := &cobra.Command{
		Use:   "auth-status",
		Short: "Return the current acli authentication status",
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
	return setAgentDocs(
		cmd,
		"Return the current Jira authentication status for the acli tool.",
		"(none)",
		`{"status":"authenticated|unauthenticated|error"}`,
		`vibecodeway-cli auth-status`,
	)
}

func newGetAISettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai-settings-get",
		Short: "Get AI configuration metadata including source (env or db)",
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
	return setAgentDocs(
		cmd,
		"Get AI configuration metadata. source is env or db. Keys are never included — only whether they are set.",
		"(none)",
		`{"provider":"string","hasKey":"bool","hasOpenAIKey":"bool","hasAnthropicKey":"bool","model":"string","baseUrl":"string","source":"env|db","updatedAt":"RFC3339"}`,
		`vibecodeway-cli ai-settings-get`,
	)
}

func newSetAISettingsCmd() *cobra.Command {
	var provider, apiKey, anthropicKey, model, baseURL string
	cmd := &cobra.Command{
		Use:   "ai-settings-set",
		Short: "Set AI configuration (provider, keys, model, baseURL)",
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
	return setAgentDocs(
		cmd,
		"Set AI configuration. Keys are stored in plain text in the database. Any flag left empty retains its current value.",
		"--provider openai|anthropic|local (optional)\n--api-key STRING (optional)\n--anthropic-key STRING (optional)\n--model STRING (optional)\n--base-url STRING (optional)",
		`{"provider":"string","hasKey":"bool","model":"string","baseUrl":"string",...}`,
		`vibecodeway-cli ai-settings-set --provider openai --model gpt-4o`,
	)
}
