package store

// Context is a named bucket that aggregates data from multiple providers.
// Provider-specific fields live in their respective provider tables.
type Context struct {
	ID         string  `json:"id"`
	Key        string  `json:"key"`
	Status     string  `json:"status"`
	OrgID      string  `json:"orgId"`
	CreatedAt  string  `json:"createdAt"`
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

	// AI-proposed workflow state — updated on every refresh.
	AIProposedStatus   string  `json:"aiProposedStatus"`
	AIProposedComment  *string `json:"aiProposedComment,omitempty"`
	AIProposedNextStep *string `json:"aiProposedNextStep,omitempty"`
	AIProposedAt       *string `json:"aiProposedAt,omitempty"`

	// User overrides — NULL means "use AI proposal".
	UserStatusOverride    *string `json:"userStatusOverride,omitempty"`
	UserCommentOverride   *string `json:"userCommentOverride,omitempty"`
	UserNextStepOverride  *string `json:"userNextStepOverride,omitempty"`
	UserOverrideUpdatedAt *string `json:"userOverrideUpdatedAt,omitempty"`
}

// EffectiveStatus returns the status the UI should display.
// needs_attention from AI always wins; otherwise user override takes precedence.
func (c *Context) EffectiveStatus() string {
	if c.AIProposedStatus == "needs_attention" {
		return "needs_attention"
	}
	if c.UserStatusOverride != nil && *c.UserStatusOverride != "" {
		return *c.UserStatusOverride
	}
	if c.AIProposedStatus != "" {
		return c.AIProposedStatus
	}
	return "not_started"
}

// EffectiveComment returns the comment the UI should display.
func (c *Context) EffectiveComment() string {
	if c.UserCommentOverride != nil && *c.UserCommentOverride != "" {
		return *c.UserCommentOverride
	}
	if c.AIProposedComment != nil {
		return *c.AIProposedComment
	}
	return ""
}

// EffectiveNextStep returns the next-step suggestion the UI should display.
func (c *Context) EffectiveNextStep() string {
	if c.UserNextStepOverride != nil && *c.UserNextStepOverride != "" {
		return *c.UserNextStepOverride
	}
	if c.AIProposedNextStep != nil {
		return *c.AIProposedNextStep
	}
	return ""
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

// OpenQuestion is an unanswered question extracted from context content by the AI agent.
type OpenQuestion struct {
	ID        string  `json:"id"`
	ContextID string  `json:"contextId"`
	Question  string  `json:"question"`
	Status    string  `json:"status"` // "open" | "answered" | "dismissed"
	Answer    *string `json:"answer,omitempty"`
	Source    string  `json:"source"` // "ai" | "user"
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	ClosedAt  *string `json:"closedAt,omitempty"`
}

// WorkflowProposal is one AI-generated workflow status proposal stored for history.
type WorkflowProposal struct {
	ID                   string  `json:"id"`
	ContextID            string  `json:"contextId"`
	ProposedStatus       string  `json:"proposedStatus"`
	ProposedComment      *string `json:"proposedComment,omitempty"`
	ProposedNextStep     *string `json:"proposedNextStep,omitempty"`
	UserStatusOverride   *string `json:"userStatusOverride,omitempty"`
	UserCommentOverride  *string `json:"userCommentOverride,omitempty"`
	UserNextStepOverride *string `json:"userNextStepOverride,omitempty"`
	PrevUserNextStep     *string `json:"prevUserNextStep,omitempty"`
	ResolvedAt           *string `json:"resolvedAt,omitempty"`
	UpdatedAt            string  `json:"updatedAt"`
	CreatedAt            string  `json:"createdAt"`
}

// Organization is a named workspace that groups related contexts.
type Organization struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
