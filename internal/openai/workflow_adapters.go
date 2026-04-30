package openai

import (
	"context"

	"github.com/artschekoff/context-consolidator/internal/openquestions"
	"github.com/artschekoff/context-consolidator/internal/workflow"
)

// --- NextStep adapter ---

// NextStepAdapter adapts *Client to the workflow.NextStepAIClient interface.
type NextStepAdapter struct{ client *Client }

// AsNextStepClient returns an adapter satisfying workflow.NextStepAIClient.
// A nil receiver is acceptable: callers should check for nil.
func (c *Client) AsNextStepClient() workflow.NextStepAIClient {
	if c == nil {
		return nil
	}
	return &NextStepAdapter{client: c}
}

// ChatCompletion implements workflow.NextStepAIClient.
func (a *NextStepAdapter) ChatCompletion(ctx context.Context, messages []workflow.NextStepMessage) (*workflow.NextStepResponse, error) {
	sdkMessages := make([]Message, 0, len(messages))
	for _, m := range messages {
		sdkMessages = append(sdkMessages, Message{Role: m.Role, Content: m.Content})
	}
	resp, err := a.client.ChatCompletion(ctx, sdkMessages)
	if err != nil {
		return nil, err
	}
	return &workflow.NextStepResponse{Content: resp.Content}, nil
}

// --- OpenQuestions adapter ---

// OpenQuestionsAdapter adapts *Client to the openquestions.AIClient interface.
type OpenQuestionsAdapter struct{ client *Client }

// AsOpenQuestionsClient returns an adapter satisfying openquestions.AIClient.
// A nil receiver is acceptable: callers should check for nil.
func (c *Client) AsOpenQuestionsClient() openquestions.AIClient {
	if c == nil {
		return nil
	}
	return &OpenQuestionsAdapter{client: c}
}

// ChatCompletion implements openquestions.AIClient.
func (a *OpenQuestionsAdapter) ChatCompletion(ctx context.Context, messages []openquestions.AIMessage) (*openquestions.AIResponse, error) {
	sdkMessages := make([]Message, 0, len(messages))
	for _, m := range messages {
		sdkMessages = append(sdkMessages, Message{Role: m.Role, Content: m.Content})
	}
	resp, err := a.client.ChatCompletion(ctx, sdkMessages)
	if err != nil {
		return nil, err
	}
	return &openquestions.AIResponse{
		Content:          resp.Content,
		Model:            resp.Model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
	}, nil
}
