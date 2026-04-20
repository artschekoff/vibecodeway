// Package openai wraps the OpenAI chat-completion API with a lazy per-call settings
// resolver (env vars first, encrypted DB row second) and a single ChatCompletion helper
// used by the contextlog service.
package openai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
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

// Source reports where the API key came from for a given call.
type Source string

const (
	SourceEnv  Source = "env"
	SourceDB   Source = "db"
	SourceNone Source = "none"
)

// ErrNotConfigured is returned by ChatCompletion when no API key is available
// either from the environment or the local database.
var ErrNotConfigured = errors.New("openai not configured — set OPENAI_API_KEY or save a key in Settings")

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

// PassphraseFn returns the current Slack passphrase used to decrypt ai_settings.openai_key_enc.
// Returning an empty string means "no passphrase available"; the DB key path is then skipped.
type PassphraseFn func() string

// Client performs chat completions against OpenAI.
// Settings are resolved on every call so the user can change them at runtime without a restart.
type Client struct {
	store      *appstore.Store
	passphrase PassphraseFn
	logger     *zap.Logger
}

// NewClient wires a Client that pulls settings from env vars first and from the encrypted
// ai_settings row second. The passphrase callback is invoked lazily per request.
func NewClient(store *appstore.Store, passphrase PassphraseFn, logger *zap.Logger) *Client {
	return &Client{store: store, passphrase: passphrase, logger: logger}
}

// DescribeSettings returns the visible AI settings (without the raw key) and the source
// from which the key would be picked if ChatCompletion were called right now.
func (c *Client) DescribeSettings(ctx context.Context) (*appstore.AISettings, Source, error) {
	dbSettings, err := c.store.GetAISettings(ctx)
	if err != nil {
		return nil, SourceNone, fmt.Errorf("loading ai settings: %w", err)
	}
	envKey := strings.TrimSpace(os.Getenv(EnvAPIKey))
	src := SourceNone
	switch {
	case envKey != "":
		src = SourceEnv
	case dbSettings.HasKey:
		src = SourceDB
	}

	effective := *dbSettings
	if envModel := strings.TrimSpace(os.Getenv(EnvModel)); envModel != "" {
		effective.Model = envModel
	}
	if effective.Model == "" {
		effective.Model = contextlog.DefaultModel
	}
	if envBase := strings.TrimSpace(os.Getenv(EnvBaseURL)); envBase != "" {
		effective.BaseURL = envBase
	}
	effective.HasKey = src != SourceNone
	return &effective, src, nil
}

// ChatCompletion sends messages to OpenAI and returns the assistant reply plus metadata.
// Returns ErrNotConfigured when no API key is resolvable.
func (c *Client) ChatCompletion(ctx context.Context, messages []Message) (*Response, error) {
	apiKey, source, err := c.resolveAPIKey(ctx)
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, ErrNotConfigured
	}

	dbSettings, _, err := c.DescribeSettings(ctx)
	if err != nil {
		return nil, err
	}

	cfg := openai.DefaultConfig(apiKey)
	if dbSettings.BaseURL != "" {
		cfg.BaseURL = strings.TrimRight(dbSettings.BaseURL, "/") + "/v1"
		// Users commonly paste the full /v1 URL; normalise to avoid /v1/v1.
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
		Model:    dbSettings.Model,
		Messages: sdkMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai returned no choices")
	}
	out := &Response{
		Content: resp.Choices[0].Message.Content,
		Model:   dbSettings.Model,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Source: source,
	}
	c.logger.Info("openai.chat_completion",
		zap.String("model", out.Model),
		zap.Int("promptTokens", out.Usage.PromptTokens),
		zap.Int("completionTokens", out.Usage.CompletionTokens),
		zap.String("source", string(source)),
	)
	return out, nil
}

// resolveAPIKey checks env first, then the encrypted DB row.
func (c *Client) resolveAPIKey(ctx context.Context) (string, Source, error) {
	if k := strings.TrimSpace(os.Getenv(EnvAPIKey)); k != "" {
		return k, SourceEnv, nil
	}
	if c.passphrase == nil {
		return "", SourceNone, nil
	}
	pass := c.passphrase()
	if pass == "" {
		return "", SourceNone, nil
	}
	key, err := c.store.GetAIOpenAIKey(ctx, pass)
	if errors.Is(err, appstore.ErrAIKeyNotSet) {
		return "", SourceNone, nil
	}
	if err != nil {
		return "", SourceNone, err
	}
	return key, SourceDB, nil
}
