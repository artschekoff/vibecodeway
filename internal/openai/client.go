// Package openai wraps the OpenAI chat-completion API with a lazy per-call settings
// resolver (env vars first, DB row second) and a single ChatCompletion helper
// used by the contextlog service.
package openai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// Env vars consulted before the DB-backed settings row.
const (
	EnvAPIKey  = "OPENAI_API_KEY"
	EnvModel   = "OPENAI_MODEL"
	EnvBaseURL = "OPENAI_BASE_URL"
)

// DefaultOpenAIModel is used when the model field is empty and provider is openai.
const DefaultOpenAIModel = "gpt-4o"

// Source reports where the API key came from for a given call.
type Source string

const (
	SourceEnv  Source = "env"
	SourceDB   Source = "db"
	SourceNone Source = "none"
)

// ErrNotConfigured is returned by ChatCompletion when no API key is available
// either from the environment or the local database.
var ErrNotConfigured = errors.New("llm not configured — set an API key in Settings")

// Message mirrors a chat-completion message at the package boundary.
type Message struct {
	Role    string
	Content string
}

// Usage mirrors the token usage returned by OpenAI.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Response bundles the completion text with metadata for logging.
type Response struct {
	Content string
	Model   string
	Usage   Usage
	Source  Source
}

// Client performs chat completions against OpenAI.
// Settings are resolved on every call so the user can change them at runtime without a restart.
type Client struct {
	store  *appstore.Store
	logger *zap.Logger
}

// NewClient wires a Client that pulls settings from env vars first and from the DB row second.
func NewClient(store *appstore.Store, logger *zap.Logger) *Client {
	return &Client{store: store, logger: logger}
}

// DescribeSettings returns the visible AI settings (without the raw key) and the source
// from which the key would be picked if ChatCompletion were called right now.
func (c *Client) DescribeSettings(ctx context.Context) (*appstore.AISettings, Source, error) {
	dbSettings, err := c.store.GetAISettings(ctx)
	if err != nil {
		return nil, SourceNone, fmt.Errorf("loading ai settings: %w", err)
	}

	effective := *dbSettings
	src := SourceNone

	switch dbSettings.Provider {
	case appstore.ProviderAnthropic:
		envKey := strings.TrimSpace(os.Getenv(EnvAnthropicAPIKey))
		switch {
		case envKey != "":
			src = SourceEnv
		case dbSettings.HasAnthropicKey:
			src = SourceDB
		}
		if effective.Model == "" {
			effective.Model = DefaultAnthropicModel
		}

	case appstore.ProviderLocal:
		src = SourceDB // always considered configured; base URL is what matters
		if effective.Model == "" {
			effective.Model = "llama3.2"
		}
		if envBase := strings.TrimSpace(os.Getenv(EnvBaseURL)); envBase != "" {
			effective.BaseURL = envBase
		}

	default: // openai
		envKey := strings.TrimSpace(os.Getenv(EnvAPIKey))
		switch {
		case envKey != "":
			src = SourceEnv
		case dbSettings.HasOpenAIKey:
			src = SourceDB
		}
		if envModel := strings.TrimSpace(os.Getenv(EnvModel)); envModel != "" {
			effective.Model = envModel
		}
		if effective.Model == "" {
			effective.Model = DefaultOpenAIModel
		}
		if envBase := strings.TrimSpace(os.Getenv(EnvBaseURL)); envBase != "" {
			effective.BaseURL = envBase
		}
	}

	effective.HasKey = src != SourceNone || dbSettings.Provider == appstore.ProviderLocal
	return &effective, src, nil
}

// callOpenAI dispatches to the OpenAI-compatible API (used for "openai" and "local" providers).
func (c *Client) callOpenAI(ctx context.Context, apiKey, model, baseURL string, messages []Message) (*Response, error) {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = strings.TrimRight(baseURL, "/") + "/v1"
		cfg.BaseURL = strings.Replace(cfg.BaseURL, "/v1/v1", "/v1", 1)
	}
	sdkClient := openai.NewClientWithConfig(cfg)

	sdkMessages := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		sdkMessages = append(sdkMessages, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	resp, err := sdkClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: sdkMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai returned no choices")
	}
	return &Response{
		Content: resp.Choices[0].Message.Content,
		Model:   model,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// ChatCompletion sends messages to the configured LLM provider and returns the reply plus metadata.
// Returns ErrNotConfigured when no API key is resolvable (for providers that require one).
func (c *Client) ChatCompletion(ctx context.Context, messages []Message) (*Response, error) {
	dbSettings, _, err := c.DescribeSettings(ctx)
	if err != nil {
		return nil, err
	}

	apiKey, source, err := c.resolveAPIKey(ctx, dbSettings)
	if err != nil {
		return nil, err
	}

	if apiKey == "" {
		if dbSettings.Provider == appstore.ProviderLocal {
			apiKey = "local" // go-openai requires non-empty; local servers typically ignore it
		} else {
			return nil, ErrNotConfigured
		}
	}

	var resp *Response
	switch dbSettings.Provider {
	case appstore.ProviderAnthropic:
		resp, err = callAnthropic(ctx, apiKey, dbSettings.Model, messages)
	default: // openai, local
		resp, err = c.callOpenAI(ctx, apiKey, dbSettings.Model, dbSettings.BaseURL, messages)
	}
	if err != nil {
		return nil, err
	}
	resp.Source = source

	c.logger.Info("llm.chat_completion",
		zap.String("provider", dbSettings.Provider),
		zap.String("model", resp.Model),
		zap.Int("promptTokens", resp.Usage.PromptTokens),
		zap.Int("completionTokens", resp.Usage.CompletionTokens),
		zap.String("source", string(source)),
	)
	return resp, nil
}

// resolveAPIKey checks env first, then the DB row.
func (c *Client) resolveAPIKey(ctx context.Context, settings *appstore.AISettings) (string, Source, error) {
	switch settings.Provider {
	case appstore.ProviderAnthropic:
		if k := strings.TrimSpace(os.Getenv(EnvAnthropicAPIKey)); k != "" {
			return k, SourceEnv, nil
		}
		key, err := c.store.GetAIAnthropicKey(ctx)
		if errors.Is(err, appstore.ErrAIKeyNotSet) {
			return "", SourceNone, nil
		}
		if err != nil {
			return "", SourceNone, err
		}
		return key, SourceDB, nil

	case appstore.ProviderLocal:
		if k := strings.TrimSpace(os.Getenv(EnvAPIKey)); k != "" {
			return k, SourceEnv, nil
		}
		key, err := c.store.GetAIOpenAIKey(ctx)
		if errors.Is(err, appstore.ErrAIKeyNotSet) {
			return "", SourceDB, nil
		}
		if err != nil {
			return "", SourceNone, err
		}
		return key, SourceDB, nil

	default: // openai
		if k := strings.TrimSpace(os.Getenv(EnvAPIKey)); k != "" {
			return k, SourceEnv, nil
		}
		key, err := c.store.GetAIOpenAIKey(ctx)
		if errors.Is(err, appstore.ErrAIKeyNotSet) {
			return "", SourceNone, nil
		}
		if err != nil {
			return "", SourceNone, err
		}
		return key, SourceDB, nil
	}
}
