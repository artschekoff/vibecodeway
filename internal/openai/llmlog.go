package openai

import (
	"encoding/json"
	"fmt"
	"time"
)

// llmLogMessage renders a one-line, human-readable summary for the App Logs view:
//
//	"llm.chat_completion model=gpt-4o tokens=12554→516 (total 13070) in 2.1s"
func llmLogMessage(event, model string, u Usage, d time.Duration) string {
	return fmt.Sprintf("%s model=%s tokens=%d→%d (total %d) in %s",
		event, model, u.PromptTokens, u.CompletionTokens, u.TotalTokens, d.Round(time.Millisecond))
}

// requestPreview serializes the request messages to compact JSON for the log's
// `request` field (shown in full inside the log detail modal).
func requestPreview(messages []Message) string {
	b, err := json.Marshal(messages)
	if err != nil {
		return ""
	}
	return string(b)
}
