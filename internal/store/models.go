package store

// Context is a named bucket that aggregates data from multiple providers.
// Provider-specific fields live in their respective provider tables.
type Context struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Status     string `json:"status"`
	OrgID      string `json:"orgId"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	IssueCount int    `json:"issueCount,omitempty"`
	// LastItemAt is the MAX created_at across all linked items (issues, comments, threads).
	// Empty string when no items exist yet.
	LastItemAt    string `json:"lastItemAt,omitempty"`
	UnviewedCount int    `json:"unviewedCount,omitempty"`
	// Provider presence flags — true when the context has at least one configured source.
	HasJira  bool `json:"hasJira,omitempty"`
	HasSlack bool `json:"hasSlack,omitempty"`
	HasVCS   bool `json:"hasVcs,omitempty"`
	HasFiles bool `json:"hasFiles,omitempty"`

	// SnapshotStatus is the effective status from the latest analysis snapshot.
	// Populated from context_snapshots.status via LEFT JOIN in list queries.
	SnapshotStatus  string `json:"snapshotStatus"`
	SnapshotIsStale bool   `json:"snapshotIsStale"`
}

// WorkflowStatus is a user-configurable status option for context workflow.
// System statuses (IsSystem = true) are seeded on first run and cannot be deleted.
type WorkflowStatus struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Label     string `json:"label"`
	IsSystem  bool   `json:"isSystem"`
	CreatedAt string `json:"createdAt"`
}

// UnviewedBreakdown reports how many unread items exist per item type for a context.
type UnviewedBreakdown struct {
	Issues        int `json:"issues"`
	Comments      int `json:"comments"`
	Threads       int `json:"threads"`
	DMs           int `json:"dms"`
	PRs           int `json:"prs"`
	PRComments    int `json:"prComments"`
	Notifications int `json:"notifications"`
	Total         int `json:"total"`
}

// Notification is a system-generated event for a state change on a tracked entity.
type Notification struct {
	ID         string  `json:"id"`
	ContextID  string  `json:"contextId"`
	EntityType string  `json:"entityType"`
	EntityID   string  `json:"entityId"`
	EventType  string  `json:"eventType"`
	OldValue   string  `json:"oldValue"`
	NewValue   string  `json:"newValue"`
	CreatedAt  string  `json:"createdAt"`
	ReadAt     *string `json:"readAt,omitempty"`
}

// ContextText is a plain-text entry associated with a context.
// Multiple texts can be attached to a single context (1 context : many texts).
type ContextText struct {
	ID        string `json:"id"`
	ContextID string `json:"contextId"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	IsActive  bool   `json:"isActive"`
}

// ContextUploadedFile is a document (PDF, DOCX, RTF, TXT, ODT) whose text has been
// extracted and stored alongside the context. Mirrors ContextText but records source
// file metadata (original name, type, size) separately from the extracted text.
// FileContent holds the original binary blob and is excluded from JSON serialisation
// (tag "-") so it is never sent over the Wails IPC bridge.
type ContextUploadedFile struct {
	ID            string `json:"id"`
	ContextID     string `json:"contextId"`
	OriginalName  string `json:"originalName"`
	FileType      string `json:"fileType"`
	ExtractedText string `json:"extractedText"`
	FileContent   []byte `json:"-"`
	FileSize      int64  `json:"fileSize"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	IsActive      bool   `json:"isActive"`
}

// ContextRelation is a typed directional link from one context to another.
// TargetKey is denormalized from the target context row for display.
type ContextRelation struct {
	ID           string `json:"id"`
	ContextID    string `json:"contextId"`
	TargetID     string `json:"targetId"`
	TargetKey    string `json:"targetKey"`
	RelationType string `json:"relationType"`
	CreatedAt    string `json:"createdAt"`
}

// AISettings holds multi-provider AI configuration stored in the local database.
// The raw API keys are never persisted or serialised — only the HasKey flags are exposed.
// Provider is one of "openai", "anthropic", or "local".
type AISettings struct {
	Provider        string `json:"provider"`
	HasKey          bool   `json:"hasKey"`          // true when the active provider has a key configured
	HasOpenAIKey    bool   `json:"hasOpenAIKey"`    // true when an OpenAI key is stored
	HasAnthropicKey bool   `json:"hasAnthropicKey"` // true when an Anthropic key is stored
	Model           string `json:"model"`
	BaseURL         string `json:"baseUrl"`
	UpdatedAt       string `json:"updatedAt"`
}

// Provider constants for AISettings.Provider.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderLocal     = "local"
)

// AppSettings holds application-level preferences stored in the local database.
type AppSettings struct {
	ContextRefetchEnabled     bool   `json:"contextRefetchEnabled"`
	ContextRefetchIntervalMin int    `json:"contextRefetchIntervalMin"`
	ReviewsRefetchEnabled     bool   `json:"reviewsRefetchEnabled"`
	ReviewsRefetchIntervalMin int    `json:"reviewsRefetchIntervalMin"`
	TerminalAppPath           string `json:"terminalAppPath"`
	UpdatedAt                 string `json:"updatedAt"`
}

// SyncSession tracks one refresh run (Refresh All or per-source).
// All items fetched during the run reference this session via sync_session_id.
type SyncSession struct {
	ID           string  `json:"id"`
	ContextID    string  `json:"contextId"`
	StartedAt    string  `json:"startedAt"`
	FinishedAt   *string `json:"finishedAt,omitempty"`
	Sources      string  `json:"sources"`      // JSON array of source names, e.g. ["jira","slack"]
	ItemsFetched int     `json:"itemsFetched"`
}

// Event is one structured, append-only entry in the event log.
// Events are extracted by the AI analysis agent from sync sessions, or created
// directly by the user (source_type = "user", sync_session_id = nil).
type Event struct {
	ID            string  `json:"id"`
	ContextID     string  `json:"contextId"`
	SyncSessionID *string `json:"syncSessionId,omitempty"`
	Type          string  `json:"type"`       // see EventType* constants
	Timestamp     string  `json:"timestamp"`  // original event time (from source)
	SourceType    string  `json:"sourceType"` // see EventSource* constants
	SourceID      string  `json:"sourceId"`
	Author        string  `json:"author"`
	Summary       string  `json:"summary"`      // AI-generated 1-2 sentence summary
	RawReference  string  `json:"rawReference"` // original text or URL
	CreatedAt     string  `json:"createdAt"`
}

// Event type constants — mirrors the plan's event taxonomy.
const (
	EventTypeRequirementStated    = "requirement_stated"
	EventTypeRequirementConfirmed = "requirement_confirmed"
	EventTypeRequirementRejected  = "requirement_rejected"
	EventTypeOpenQuestionRaised   = "open_question_raised"
	EventTypeOpenQuestionAnswered = "open_question_answered"
	EventTypeDecisionMade         = "decision_made"
	EventTypeBlockerIdentified    = "blocker_identified"
	EventTypeBlockerResolved      = "blocker_resolved"
	EventTypeWorkCompleted        = "work_completed"
	EventTypeStatusUpdate         = "status_update"
	EventTypeCurrentStepOverride       = "current_step_override"
	EventTypeNextStepRecommendation    = "next_step_recommendation"
	EventTypeUserNote                  = "user_note"
)

// Event source type constants.
const (
	EventSourceJira   = "jira"
	EventSourceSlack  = "slack"
	EventSourceGitHub = "github"
	EventSourceChat   = "chat"
	EventSourceUser   = "user"
)

// ContextSnapshot is the AI-maintained state document for a context.
// It has three markdown sections derived from the full event log, plus
// a current_step that can be overridden by the user.
type ContextSnapshot struct {
	ContextID             string  `json:"contextId"`
	CurrentStepDesc       string  `json:"currentStepDesc"`
	CurrentStepConfidence string  `json:"currentStepConfidence"` // "high" | "medium" | "low"
	CurrentStepFrom       string  `json:"currentStepFrom"`       // JSON array of event IDs
	CurrentStepOverridden bool    `json:"currentStepOverridden"`
	SectionOperational    string  `json:"sectionOperational"`
	SectionNarrative      string  `json:"sectionNarrative"`
	SectionEngineering    string  `json:"sectionEngineering"`
	Status string `json:"status"` // slug from workflow_statuses, e.g. "in_progress"

	// AI-generated next-step recommendations by time horizon (rebuilt on each analysis).
	AINextStepsShort string `json:"aiNextStepsShort"` // ~1 day
	AINextStepsMid   string `json:"aiNextStepsMid"`   // ~1 week
	AINextStepsLong  string `json:"aiNextStepsLong"`  // ~1 month

	// User-written next-step notes by time horizon (editable any time, no re-analysis needed).
	// These are passed to the AI as hints during analysis.
	UserNextStepsShort string `json:"userNextStepsShort"`
	UserNextStepsMid   string `json:"userNextStepsMid"`
	UserNextStepsLong  string `json:"userNextStepsLong"`

	LastAnalyzedSessionID *string `json:"lastAnalyzedSessionId,omitempty"`
	AnalyzedAt            *string `json:"analyzedAt,omitempty"`
	IsStale               bool    `json:"isStale"`
	Version               int     `json:"version"`
}

// AIPrompt is a user-editable prompt template stored in the database.
// Each prompt uses {{placeholder}} syntax for dynamic values substituted at call time.
type AIPrompt struct {
	ID          string              `json:"id"`
	Label       string              `json:"label"`
	Body        string              `json:"body"`
	DefaultBody string              `json:"defaultBody"`
	Placeholders []PromptPlaceholder `json:"placeholders"`
	UpdatedAt   string              `json:"updatedAt"`
}

// PromptPlaceholder describes a single interpolation variable available in a prompt template.
type PromptPlaceholder struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Organization is a named workspace that groups related contexts.
type Organization struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
