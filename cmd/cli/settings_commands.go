package main

import (
	"context"
	"fmt"
	"os"

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
		Long: `Set AI configuration. Passphrase is read from CONTEXT_CONSOLIDATOR_PASSPHRASE
env var and is required when --api-key or --anthropic-key is non-empty.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			passphrase := os.Getenv(passphraseEnvVar)
			if (apiKey != "" || anthropicKey != "") && passphrase == "" {
				return fmt.Errorf("set %s first (used to encrypt the API key)", passphraseEnvVar)
			}
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
				Passphrase:   passphrase,
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
	cmd.Flags().StringVar(&apiKey, "api-key", "", "OpenAI API key (encrypted at rest)")
	cmd.Flags().StringVar(&anthropicKey, "anthropic-key", "", "Anthropic API key (encrypted at rest)")
	cmd.Flags().StringVar(&model, "model", "", "Model name")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Custom base URL (for OpenAI proxy or local endpoints)")
	return cmd
}
