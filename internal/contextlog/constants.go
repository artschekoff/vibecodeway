// Package contextlog records AI-generated and user-authored summaries of context changes,
// maintains a rolling consolidated summary per context, and powers the chat-with-context UI.
package contextlog

// Trigger values stored in context_logs.trigger.
const (
	TriggerRefresh = "refresh" // RefreshContext call
	TriggerReload  = "reload"  // ReloadAllContexts (per-context)
	TriggerManual  = "manual"  // user-authored entry
	TriggerChat    = "chat"    // generated from a chat apply action (reserved)
	TriggerInit    = "init"    // consolidated-log bootstrap
	TriggerAgent   = "agent"   // agent-authored entry via MCP
)

// Chat roles stored in context_log_chat_messages.role.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// ConsolidatedUpdatedBy values.
const (
	UpdatedByAI   = "ai"
	UpdatedByUser = "user"
)

// Default values.
const (
	DefaultModel = "gpt-4o-mini"
	// ChatHistoryLimit bounds how many recent chat messages are included in prompts.
	ChatHistoryLimit = 20
)
