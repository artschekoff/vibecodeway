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
func callAnthropic(ctx context.Context, apiKey, model string, messages []Message, tools []ToolDef) (*Response, error) {
	client := anthropicSDK.NewClient(option.WithAPIKey(apiKey))

	systemText, chatMessages := toAnthropicMessages(messages)

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
	if len(tools) > 0 {
		params.Tools = toAnthropicTools(tools)
	}

	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat completion: %w", err)
	}

	out := &Response{
		Model: string(resp.Model),
		Usage: Usage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			out.Content += block.Text
		case "tool_use":
			tu := block.AsToolUse()
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:        tu.ID,
				Name:      tu.Name,
				Arguments: parseAnthropicToolArgs(tu.Input),
			})
		}
	}
	return out, nil
}
