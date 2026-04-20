package openai

import (
	"context"
	"errors"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
)

// ContextLogAdapter adapts *Client to the contextlog.AIClient interface, translating
// between the package-local Message/Response types and the ones expected by the
// contextlog service.
type ContextLogAdapter struct {
	client *Client
}

// AsContextLogClient returns an adapter satisfying contextlog.AIClient.
// A nil receiver (when OpenAI is entirely disabled at the app level) is acceptable:
// callers should check for nil and pass nil to contextlog.NewService.
func (c *Client) AsContextLogClient() contextlog.AIClient {
	if c == nil {
		return nil
	}
	return &ContextLogAdapter{client: c}
}

// ChatCompletion implements contextlog.AIClient.
func (a *ContextLogAdapter) ChatCompletion(ctx context.Context, messages []contextlog.AIMessage) (*contextlog.AIResponse, error) {
	sdkMessages := make([]Message, 0, len(messages))
	for _, m := range messages {
		sdkMessages = append(sdkMessages, Message{Role: m.Role, Content: m.Content})
	}
	resp, err := a.client.ChatCompletion(ctx, sdkMessages)
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return nil, contextlog.ErrAINotConfigured
		}
		return nil, err
	}
	return &contextlog.AIResponse{
		Content:          resp.Content,
		Model:            resp.Model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
	}, nil
}
