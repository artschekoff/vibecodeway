package openai

import (
	"strings"
	"testing"
	"time"
)

func TestLLMLogMessage(t *testing.T) {
	got := llmLogMessage("llm.chat_completion", "gpt-4o",
		Usage{PromptTokens: 12554, CompletionTokens: 516, TotalTokens: 13070},
		2100*time.Millisecond)
	for _, want := range []string{"llm.chat_completion", "gpt-4o", "12554", "516", "13070", "2.1s"} {
		if !strings.Contains(got, want) {
			t.Fatalf("message %q missing %q", got, want)
		}
	}
}

func TestRequestPreview(t *testing.T) {
	got := requestPreview([]Message{{Role: "user", Content: "hello"}})
	if !strings.Contains(got, "user") || !strings.Contains(got, "hello") {
		t.Fatalf("preview missing role/content: %q", got)
	}
}
