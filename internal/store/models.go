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
	// SnapshotStatusComment is the comment from the latest status log entry.
	// Denormalized from context_status_log in list/get queries for tooltip display.
	SnapshotStatusComment string `json:"snapshotStatusComment,omitempty"`
	// Importance is the average of the numeric scores of contained issues' priorities
	// (Highest=5 ... Lowest=1). Nil when no contained issue has a mapped priority.
	Importance *float64 `json:"importance,omitempty"`
}

// ContextUpdateAction is one AI-proposed action to apply to a context
// in response to a user's free-form Quick Update text. InterpretContextUpdate
// returns a slice of these — the user reviews and confirms which to apply.
type ContextUpdateAction struct {
	// One of: set_status, add_event, add_ticket, add_repo, add_relation
	Action string `json:"action"`
	// For action="add_event": one of the EventType* constants
	EventType string `json:"eventType,omitempty"`
	// For action="add_event": 1-2 sentence summary
	Summary string `json:"summary,omitempty"`
	// For action="set_status": workflow status slug
	StatusSlug string `json:"statusSlug,omitempty"`
	// For action="set_status": reason for the change
	StatusComment string `json:"statusComment,omitempty"`
	// For action="add_ticket": Jira issue keys to link to the context
	JiraKeys []string `json:"jiraKeys,omitempty"`
	// For action="add_repo": VCS platform (e.g. "github")
	Platform string `json:"platform,omitempty"`
	// For action="add_repo": "owner/repo" slug
	OwnerRepo string `json:"ownerRepo,omitempty"`
	// For action="add_repo": branch name to link
	Branch string `json:"branch,omitempty"`
	// For action="add_repo": optional human-readable label for the repo link
	RepoLabel string `json:"repoLabel,omitempty"`
	// For action="add_relation": the key of the other context to relate to
	TargetContextKey string `json:"targetContextKey,omitempty"`
	// For action="add_relation": the relation type (e.g. "blocks", "related_to")
	RelationType string `json:"relationType,omitempty"`
	// Human-readable description shown as preview to the user
	Description string `json:"description"`
}

// WorkflowStatus is a user-configurable status option for context workflow.
// System statuses (IsSystem = true) are seeded on first run and cannot be deleted.
type WorkflowStatus struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Label     string `json:"label"`
	AIHint    string `json:"aiHint"`
	Color     string `json:"color"`
	IsSystem  bool   `json:"isSystem"`
	CreatedAt string `json:"createdAt"`
}

// StatusLogEntry is one node in the per-context status history log.
// PrevID links to the entry that preceded this one (nil for the first entry).
type StatusLogEntry struct {
	ID         string  `json:"id"`
	ContextID  string  `json:"contextId"`
	StatusSlug string  `json:"statusSlug"`
	Comment    string  `json:"comment"`
	Source     string  `json:"source"` // "ai" | "user"
	PrevID     *string `json:"prevId,omitempty"`
	CreatedAt  string  `json:"createdAt"`
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
	ID         string `json:"id"`
	ContextID  string `json:"contextId"`
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
	EventType  string `json:"eventType"`
	OldValue   string `json:"oldValue"`
	NewValue   string `json:"newValue"`
	// Note is the matched rule's text, copied at fire time. "" when no rule matched.
	Note      string  `json:"note"`
	CreatedAt string  `json:"createdAt"`
	ReadAt    *string `json:"readAt,omitempty"`
}

// NotificationRule is a user-defined watch on a context: when an event of
// EventType arrives whose new value matches MatchValue, the notification it
// produces carries Note. Created by the Ask-AI companion, not by a settings
// screen — the companion is the interface.
type NotificationRule struct {
	ID         string `json:"id"`
	ContextID  string `json:"contextId"`
	EventType  string `json:"eventType"`
	MatchValue string `json:"matchValue"` // "" matches any new value
	Note       string `json:"note"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"createdAt"`
}

// ActivityFeedItem is a notification joined with its context key, for the
// cross-context activity feed. Read and unread are both included.
type ActivityFeedItem struct {
	ID         string `json:"id"`
	ContextID  string `json:"contextId"`
	ContextKey string `json:"contextKey"`
	EntityType string `json:"entityType"`
	EventType  string `json:"eventType"`
	OldValue   string `json:"oldValue"`
	NewValue   string `json:"newValue"`
	// Note is the notification-rule text the user attached to this event, or "".
	Note      string `json:"note"`
	CreatedAt string `json:"createdAt"`
}

// ContextEventHook is a per-context automation: when an event of EventCategory
// is newly detected, run the action of ActionType.
type ContextEventHook struct {
	ID            string `json:"id"`
	ContextID     string `json:"contextId"`
	EventCategory string `json:"eventCategory"` // jira_update | pr_update | code_review
	ActionType    string `json:"actionType"`    // os_notification | run_script
	ScriptCommand string `json:"scriptCommand"` // empty unless ActionType == run_script
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// GlobalEventHook is a default hook applied to every context for an event
// category, unless that context defines its own hook(s) for the category.
type GlobalEventHook struct {
	ID            string `json:"id"`
	EventCategory string `json:"eventCategory"`
	ActionType    string `json:"actionType"`
	ScriptCommand string `json:"scriptCommand"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
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

// ContextSearchHit is a single cross-org search result for the global search
// palette. Summary is the primary Jira issue's summary (empty when none).
type ContextSearchHit struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	OrgID   string `json:"orgId"`
	OrgName string `json:"orgName"`
	Summary string `json:"summary"`
}

// ContextRelation is a typed directional link from one context to another.
// TargetKey, TargetOrgID, and TargetOrgName are denormalized from the target context row for display.
type ContextRelation struct {
	ID            string `json:"id"`
	ContextID     string `json:"contextId"`
	TargetID      string `json:"targetId"`
	TargetKey     string `json:"targetKey"`
	TargetOrgID   string `json:"targetOrgId"`
	TargetOrgName string `json:"targetOrgName"`
	RelationType  string `json:"relationType"`
	CreatedAt     string `json:"createdAt"`
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
	ContextRefetchEnabled        bool    `json:"contextRefetchEnabled"`
	ContextRefetchIntervalMin    int     `json:"contextRefetchIntervalMin"`
	ReviewsRefetchEnabled        bool    `json:"reviewsRefetchEnabled"`
	ReviewsRefetchIntervalMin    int     `json:"reviewsRefetchIntervalMin"`
	TerminalAppPath              string  `json:"terminalAppPath"`
	CodingAgentApp               string  `json:"codingAgentApp"`
	AutoLinkReposOnContextCreate bool    `json:"autoLinkReposOnContextCreate"`
	EditorSettings               string  `json:"editorSettings"`       // opaque JSON blob owned by the frontend
	PRStaleThresholdDays         int     `json:"prStaleThresholdDays"` // PR considered stale after N days awaiting review
	WorkdayHours                 float64 `json:"workdayHours"`         // theoretical workday length, hours
	TaskHours                    float64 `json:"taskHours"`            // theoretical cost of one closed task, hours
	// StatsAuthorName is the user's own Jira display name, used to attribute
	// synced comments to them for comment_authored stat-event capture.
	// Empty (the default) means capture is skipped entirely.
	StatsAuthorName string `json:"statsAuthorName"`
	UpdatedAt       string `json:"updatedAt"`
}

// SyncSession tracks one refresh run (Refresh All or per-source).
// All items fetched during the run reference this session via sync_session_id.
type SyncSession struct {
	ID           string  `json:"id"`
	ContextID    string  `json:"contextId"`
	StartedAt    string  `json:"startedAt"`
	FinishedAt   *string `json:"finishedAt,omitempty"`
	Sources      string  `json:"sources"` // JSON array of source names, e.g. ["jira","slack"]
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
	EventTypeRequirementStated      = "requirement_stated"
	EventTypeRequirementConfirmed   = "requirement_confirmed"
	EventTypeRequirementRejected    = "requirement_rejected"
	EventTypeOpenQuestionRaised     = "open_question_raised"
	EventTypeOpenQuestionAnswered   = "open_question_answered"
	EventTypeDecisionMade           = "decision_made"
	EventTypeBlockerIdentified      = "blocker_identified"
	EventTypeBlockerResolved        = "blocker_resolved"
	EventTypeWorkCompleted          = "work_completed"
	EventTypeStatusUpdate           = "status_update"
	EventTypeCurrentStepOverride    = "current_step_override"
	EventTypeNextStepRecommendation = "next_step_recommendation"
	EventTypeUserNote               = "user_note"
	EventTypeCommentAdded           = "comment_added"
	EventTypeReviewRequested        = "review_requested"
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
	ContextID             string `json:"contextId"`
	CurrentStepDesc       string `json:"currentStepDesc"`
	CurrentStepConfidence string `json:"currentStepConfidence"` // "high" | "medium" | "low"
	CurrentStepFrom       string `json:"currentStepFrom"`       // JSON array of event IDs
	CurrentStepOverridden bool   `json:"currentStepOverridden"`
	SectionOperational    string `json:"sectionOperational"`
	SectionNarrative      string `json:"sectionNarrative"`
	SectionEngineering    string `json:"sectionEngineering"`
	Status                string `json:"status"` // slug from workflow_statuses, e.g. "in_progress"
	StatusComment         string `json:"-"`      // transient: AI's one-sentence reason for the chosen status; never serialised to frontend

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
	AnalyzedThrough       *string `json:"analyzedThrough,omitempty"`
	IsStale               bool    `json:"isStale"`
	Version               int     `json:"version"`

	// LatestStatusLogEntry is the most recent status log node, populated by GetSnapshot.
	LatestStatusLogEntry *StatusLogEntry `json:"latestStatusLogEntry,omitempty"`

	// AI-predicted effort remaining and completion date (updated on each AnalyzeContext run).
	EstimatedHours          *float64 `json:"estimatedHours,omitempty"`
	EstimatedCompletionDate *string  `json:"estimatedCompletionDate,omitempty"`
	EstimationConfidence    string   `json:"estimationConfidence"` // "high" | "medium" | "low" | ""
	EstimationBasis         string   `json:"estimationBasis"`
}

// EstimationLog records one change to a context's estimated effort.
// trigger_type values: "initial" | "re-analysis" | "requirement_change"
type EstimationLog struct {
	ID                        string   `json:"id"`
	ContextID                 string   `json:"contextId"`
	RecordedAt                string   `json:"recordedAt"`
	TriggerType               string   `json:"triggerType"`
	TriggerDescription        string   `json:"triggerDescription"`
	DeltaHours                *float64 `json:"deltaHours,omitempty"`
	EstimatedHoursBefore      *float64 `json:"estimatedHoursBefore,omitempty"`
	EstimatedHoursAfter       *float64 `json:"estimatedHoursAfter,omitempty"`
	EstimatedCompletionBefore *string  `json:"estimatedCompletionBefore,omitempty"`
	EstimatedCompletionAfter  *string  `json:"estimatedCompletionAfter,omitempty"`
	EstimationBasis           string   `json:"estimationBasis"`
}

// PersonAssignment holds the active (is_current) version of one person's task assignment.
// Body always mirrors the is_current ReasoningChainStep's output_state for entity_type="person_assignment".
type PersonAssignment struct {
	ID              string  `json:"id"`
	ContextID       string  `json:"contextId"`
	Name            string  `json:"name"`
	Body            string  `json:"body"`
	GeneratedAt     *string `json:"generatedAt,omitempty"`
	AnalyzedThrough *string `json:"analyzedThrough,omitempty"`
	IsOutdated      bool    `json:"isOutdated"`
	UpdatedAt       string  `json:"updatedAt"`
}

// ContextPlan is an AI-generated implementation-plan markdown document tied to a
// context and the person assignment it was generated for.
type ContextPlan struct {
	ID                 string `json:"id"`
	ContextID          string `json:"contextId"`
	PersonAssignmentID string `json:"personAssignmentId"`
	Body               string `json:"body"`
	CreatedAt          string `json:"createdAt"`
}

// ReasoningChainStep is one step in a generic reasoning chain.
// entity_type = "person_assignment" | "context"
// input_state/output_state hold the domain-specific payload (markdown body, JSON events, etc.)
type ReasoningChainStep struct {
	ID                string   `json:"id"`
	EntityType        string   `json:"entityType"`
	EntityID          string   `json:"entityId"`
	StepIndex         int      `json:"stepIndex"`
	SourceType        string   `json:"sourceType"`
	MessageID         string   `json:"messageId,omitempty"`
	MessageContent    string   `json:"messageContent"`
	UserInstruction   string   `json:"userInstruction,omitempty"`
	InputState        string   `json:"inputState"`
	OutputState       string   `json:"outputState"`
	CompressedSummary string   `json:"compressedSummary"`
	EventsSnapshot    string   `json:"eventsSnapshot"`
	PromptVersion     string   `json:"promptVersion"`
	LLMPrompt         string   `json:"llmPrompt,omitempty"` // rendered prompt sent to LLM, loaded via JOIN from step_prompts table
	QualityScore      *float64 `json:"qualityScore,omitempty"`
	QualityNotes      string   `json:"qualityNotes,omitempty"`
	IsCurrent         bool     `json:"isCurrent"`
	CreatedAt         string   `json:"createdAt"`
}

// PersonAssignmentVersion is one versioned snapshot of a PersonAssignment's output body.
// Exactly one version per person_assignment_id should have IsActive = true (maintained by application layer).
type PersonAssignmentVersion struct {
	ID                 string `json:"id"`
	PersonAssignmentID string `json:"personAssignmentId"`
	Body               string `json:"body"`
	LLMPrompt          string `json:"llmPrompt,omitempty"`
	IsActive           bool   `json:"isActive"`
	CreatedAt          string `json:"createdAt"`
}

// MessageEmbedding stores a float32 embedding vector for a message as a raw BLOB.
type MessageEmbedding struct {
	MessageID string    `json:"messageId"`
	Embedding []float32 `json:"-"` // not serialised to frontend
	Model     string    `json:"model"`
	CreatedAt string    `json:"createdAt"`
}

// DefaultEmbeddingModel is the OpenAI model used to produce message embeddings.
const DefaultEmbeddingModel = "text-embedding-3-small"

// Keybinding is a user-configurable keyboard shortcut.
// modifier_keys is a comma-separated list of: "cmd", "shift", "ctrl", "alt".
// key_code is a lowercase key name: "c", "enter", "space", "f1"–"f12", etc.
type Keybinding struct {
	ID                  string `json:"id"`
	Command             string `json:"command"`
	Label               string `json:"label"`
	Description         string `json:"description"`
	ModifierKeys        string `json:"modifierKeys"` // e.g. "cmd,shift"
	KeyCode             string `json:"keyCode"`      // e.g. "c"
	Sequence            string `json:"sequence"`
	Object              string `json:"object"`
	Action              string `json:"action"`
	Scope               string `json:"scope"`
	DefaultModifierKeys string `json:"defaultModifierKeys"`
	DefaultKeyCode      string `json:"defaultKeyCode"`
	DefaultSequence     string `json:"defaultSequence"`
	UpdatedAt           string `json:"updatedAt"`
}

// TerminalCommand is a user-configured command shown in the editor terminal's
// command selector. Command may contain placeholder tokens (e.g. <context-name>)
// substituted at insert time.
type TerminalCommand struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Command string `json:"command"`
	// SendEnter: when true, selecting the command also sends Enter so it runs.
	SendEnter bool   `json:"sendEnter"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// AIPrompt is a user-editable prompt template stored in the database.
// Each prompt uses {{placeholder}} syntax for dynamic values substituted at call time.
type AIPrompt struct {
	ID           string              `json:"id"`
	Label        string              `json:"label"`
	Body         string              `json:"body"`
	DefaultBody  string              `json:"defaultBody"`
	Placeholders []PromptPlaceholder `json:"placeholders"`
	UpdatedAt    string              `json:"updatedAt"`
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

// Source is a row from the sources dictionary table.
type Source struct {
	ID        string `json:"id"`    // slug, e.g. "jira"
	Label     string `json:"label"` // e.g. "Jira"
	CreatedAt string `json:"createdAt"`
}

// User is an org-level person. Not linked to any specific source system.
type User struct {
	ID          string  `json:"id"`
	OrgID       *string `json:"orgId,omitempty"`
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email"`
	AvatarURL   string  `json:"avatarUrl"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// AuthorMapping links a verbatim source-system name to an org User.
type AuthorMapping struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	SourceID     string `json:"sourceId"`
	ExternalName string `json:"externalName"` // verbatim name as it appears in source data
	CreatedAt    string `json:"createdAt"`
}

// UnresolvedAuthor is one unknown author found in a context's messages.
// Returned by GetUnresolvedAuthors for the assignment modal.
type UnresolvedAuthor struct {
	SourceID     string `json:"sourceId"`
	SourceLabel  string `json:"sourceLabel"`
	RawAuthor    string `json:"rawAuthor"`
	MessageCount int    `json:"messageCount"`
}

// Message is the base abstraction for any content item from any provider.
// AuthorDisplayName and SourceLabel are populated by JOINs at query time.
type Message struct {
	ID                string  `json:"id"`
	AuthorID          *string `json:"authorId,omitempty"`
	AuthorDisplayName string  `json:"authorDisplayName,omitempty"` // joined from users
	RawAuthor         string  `json:"rawAuthor"`                   // verbatim from source
	ContextID         *string `json:"contextId,omitempty"`
	OrganizationID    *string `json:"organizationId,omitempty"`
	ParentID          *string `json:"parentId,omitempty"`
	Title             string  `json:"title"`
	Body              string  `json:"body"`
	SourceID          string  `json:"sourceId"`
	SourceLabel       string  `json:"sourceLabel,omitempty"` // joined from sources
	ExternalID        string  `json:"externalId"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	InsertedAt        string  `json:"insertedAt"`
}

// CreateUserArgs holds fields for creating a new org user.
type CreateUserArgs struct {
	OrgID       string
	DisplayName string
	Email       string
	AvatarURL   string
}

// UpsertMessageArgs holds data for creating or updating a Message record.
type UpsertMessageArgs struct {
	ContextID      string
	OrganizationID string // optional → NULL when empty
	ParentID       string // optional → NULL when empty
	AuthorID       string // optional → NULL when empty (unresolved)
	RawAuthor      string // verbatim name from source; always set when author is known
	SourceID       string // FK to sources.id, e.g. "jira"
	ExternalID     string
	Title          string
	Body           string
	CreatedAt      string // RFC3339
	UpdatedAt      string // RFC3339
}

// AITodoItem is one row of the board-wide "TODO NEXT (BY AI)" list.
type AITodoItem struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // implement | make_pr | ask_question | check_status | rerequest_pr
	Title     string `json:"title"`
	Rationale string `json:"rationale"`
	RefType   string `json:"refType"` // jira | pr | review | ""
	RefKey    string `json:"refKey"`  // e.g. JH-4922 or juna-health/juna-api#218
	// Resolved at read time (not stored): the in-app context this ref maps to,
	// so the board can deep-link into it. "" when the ref has no linked context.
	ContextID  string `json:"contextId"`
	ContextKey string `json:"contextKey"`
	Priority   string `json:"priority"` // critical | high | medium | low
	Position   int    `json:"position"`
	Status     string `json:"status"` // pending | done | dismissed
	Source     string `json:"source"` // ai | user
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// TaskCompletion is one Jira issue observed moving from an open status to a
// terminal one. CompletedAt is when the app detected it, not Jira's own
// resolution timestamp — the app never sees the latter.
type TaskCompletion struct {
	Key         string `json:"key"`
	IssueType   string `json:"issueType"`
	Org         string `json:"org"`
	Status      string `json:"status"`
	CompletedAt string `json:"completedAt"`
}
