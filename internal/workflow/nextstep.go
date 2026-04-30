package workflow

import (
	"context"
	"fmt"
	"strings"
)

// NextStepAIClient is the minimal interface needed to generate next-step text.
type NextStepAIClient interface {
	ChatCompletion(ctx context.Context, messages []NextStepMessage) (*NextStepResponse, error)
}

type NextStepMessage struct {
	Role    string
	Content string
}

type NextStepResponse struct {
	Content string
}

const nextStepSystemPrompt = `You are an engineering team assistant.
Given a context workflow status, a brief description of the situation, and optionally
the developer's previous note about what to do next, produce a single concise sentence
(max 20 words) describing the recommended next action.
Do not use bullet points or headings — just one sentence.`

// GenerateNextStep produces a next-step suggestion for the given status.
// If prevUserNextStep is non-empty the AI incorporates it into the suggestion,
// so user corrections are carried forward into future proposals.
// Falls back to a hardcoded template when ai is nil or returns an error.
func GenerateNextStep(ctx context.Context, ai NextStepAIClient, status, comment, prevUserNextStep string) string {
	if ai != nil {
		if suggestion := callAIForNextStep(ctx, ai, status, comment, prevUserNextStep); suggestion != "" {
			return suggestion
		}
	}
	return fallbackNextStep(status, comment)
}

func callAIForNextStep(ctx context.Context, ai NextStepAIClient, status, comment, prevUserNextStep string) string {
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Status: %s\n", status)
	if comment != "" {
		fmt.Fprintf(&prompt, "Situation: %s\n", comment)
	}
	if prevUserNextStep != "" {
		fmt.Fprintf(&prompt, "Developer's previous note: %s\n", prevUserNextStep)
	}
	prompt.WriteString("What is the recommended next action?")

	resp, err := ai.ChatCompletion(ctx, []NextStepMessage{
		{Role: "system", Content: nextStepSystemPrompt},
		{Role: "user", Content: prompt.String()},
	})
	if err != nil || resp == nil {
		return ""
	}
	return strings.TrimSpace(resp.Content)
}

// fallbackNextStep returns a template-based next step when AI is unavailable.
func fallbackNextStep(status, comment string) string {
	switch status {
	case "not_started":
		return "Define the technical approach and create the initial branch."
	case "scoping":
		return "Review and answer the open questions before coding begins."
	case "needs_attention":
		if strings.Contains(comment, "merge conflict") {
			return "Resolve the merge conflict and re-push the branch."
		}
		if strings.Contains(comment, "changes requested") {
			return "Address the review feedback and push an updated commit."
		}
		if strings.Contains(comment, "CI checks") {
			return "Fix the failing CI checks and push a fix commit."
		}
		return "Investigate and resolve the blocking issue."
	case "in_progress":
		return "Continue development work."
	case "ready":
		return "Merge or deploy when the team is ready."
	default:
		return "Do work manually ;)"
	}
}
