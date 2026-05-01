package openai

import (
	"context"
	"fmt"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// DefaultAnthropicModel is used when the model field is empty and provider is anthropic.
const DefaultAnthropicModel = "claude-sonnet-4-6"

// EnvAnthropicAPIKey is the environment variable consulted for the Anthropic API key.
const EnvAnthropicAPIKey = "ANTHROPIC_API_KEY"

// callAnthropic sends messages to the Anthropic Messages API and returns a Response.
// System-role messages are extracted and placed in the top-level System field as required
// by the Anthropic API (system is not a member of the messages array).
func callAnthropic(ctx context.Context, apiKey, model string, messages []Message) (*Response, error) {
	client := anthropicSDK.NewClient(option.WithAPIKey(apiKey))

	var systemText string
	var chatMessages []anthropicSDK.MessageParam

	for _, m := range messages {
		switch m.Role {
		case "system":
			if systemText != "" {
				systemText += "\n"
			}
			systemText += m.Content
		case "user":
			chatMessages = append(chatMessages, anthropicSDK.NewUserMessage(
				anthropicSDK.NewTextBlock(m.Content),
			))
		case "assistant":
			chatMessages = append(chatMessages, anthropicSDK.NewAssistantMessage(
				anthropicSDK.NewTextBlock(m.Content),
			))
		}
	}

	params := anthropicSDK.MessageNewParams{
		Model:     anthropicSDK.Model(model),
		MaxTokens: 4096,
		Messages:  chatMessages,
	}
	if systemText != "" {
		params.System = []anthropicSDK.TextBlockParam{
			{Text: systemText},
		}
	}

	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat completion: %w", err)
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content = block.Text
			break
		}
	}

	return &Response{
		Content: content,
		Model:   string(resp.Model),
		Usage: Usage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}, nil
}
