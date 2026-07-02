package openai

import (
	"context"
	"encoding/json"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// ToolDef describes a tool the model may call. Parameters is a JSON-schema object
// ({"type":"object","properties":{...},"required":[...]}) describing the arguments.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall is a model request to invoke a tool with JSON-encoded arguments.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ChatWithTools performs a single chat-completion round that may return tool calls.
// The caller (agent loop) is responsible for executing the returned ToolCalls,
// appending tool-result messages, and calling again until no tool calls remain.
func (c *Client) ChatWithTools(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
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
			apiKey = "local"
		} else {
			return nil, ErrNotConfigured
		}
	}

	resp, err := c.dispatch(ctx, dbSettings, apiKey, messages, tools)
	if err != nil {
		return nil, err
	}
	resp.Source = source

	c.logger.Info("llm.chat_with_tools",
		zap.String("provider", dbSettings.Provider),
		zap.String("model", resp.Model),
		zap.Int("promptTokens", resp.Usage.PromptTokens),
		zap.Int("completionTokens", resp.Usage.CompletionTokens),
		zap.Int("toolCalls", len(resp.ToolCalls)),
	)
	return resp, nil
}

// dispatch routes a request to the right provider implementation with optional tools.
func (c *Client) dispatch(ctx context.Context, s *appstore.AISettings, apiKey string, messages []Message, tools []ToolDef) (*Response, error) {
	switch s.Provider {
	case appstore.ProviderAnthropic:
		return callAnthropic(ctx, apiKey, s.Model, messages, tools)
	default: // openai, local
		return c.callOpenAI(ctx, apiKey, s.Model, s.BaseURL, messages, tools)
	}
}

// ── OpenAI conversions ───────────────────────────────────────────────────────

func toOpenAITools(tools []ToolDef) []openai.Tool {
	out := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		out = append(out, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		msg := openai.ChatCompletionMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:       tc.ID,
				Type:     openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		out = append(out, msg)
	}
	return out
}

// ── Anthropic conversions ────────────────────────────────────────────────────

func toAnthropicTools(tools []ToolDef) []anthropicSDK.ToolUnionParam {
	out := make([]anthropicSDK.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		var props any
		var required []string
		if t.Parameters != nil {
			props = t.Parameters["properties"]
			if r, ok := t.Parameters["required"].([]string); ok {
				required = r
			}
		}
		tp := anthropicSDK.ToolParam{
			Name:        t.Name,
			Description: anthropicSDK.String(t.Description),
			InputSchema: anthropicSDK.ToolInputSchemaParam{
				Properties: props,
				Required:   required,
			},
		}
		out = append(out, anthropicSDK.ToolUnionParam{OfTool: &tp})
	}
	return out
}

// toAnthropicMessages converts boundary messages into Anthropic message params,
// pulling out system text and merging consecutive tool-result messages into a
// single user turn (as the Anthropic API requires).
func toAnthropicMessages(messages []Message) (systemText string, out []anthropicSDK.MessageParam) {
	var pendingToolResults []anthropicSDK.ContentBlockParamUnion
	flush := func() {
		if len(pendingToolResults) > 0 {
			out = append(out, anthropicSDK.NewUserMessage(pendingToolResults...))
			pendingToolResults = nil
		}
	}

	for _, m := range messages {
		switch m.Role {
		case "system":
			if systemText != "" {
				systemText += "\n"
			}
			systemText += m.Content
		case "tool":
			pendingToolResults = append(pendingToolResults,
				anthropicSDK.NewToolResultBlock(m.ToolCallID, m.Content, false))
		case "assistant":
			flush()
			blocks := []anthropicSDK.ContentBlockParamUnion{}
			if m.Content != "" {
				blocks = append(blocks, anthropicSDK.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				var input any
				if tc.Arguments != "" {
					_ = json.Unmarshal([]byte(tc.Arguments), &input)
				}
				blocks = append(blocks, anthropicSDK.NewToolUseBlock(tc.ID, input, tc.Name))
			}
			if len(blocks) > 0 {
				out = append(out, anthropicSDK.NewAssistantMessage(blocks...))
			}
		default: // user
			flush()
			out = append(out, anthropicSDK.NewUserMessage(anthropicSDK.NewTextBlock(m.Content)))
		}
	}
	flush()
	return systemText, out
}

// parseAnthropicToolArgs re-encodes a raw tool-use input blob as a JSON string so
// it matches the OpenAI ToolCall.Arguments contract.
func parseAnthropicToolArgs(input json.RawMessage) string {
	if len(input) == 0 {
		return "{}"
	}
	return string(input)
}
