package contextlog

// ContextLog is a single change-log entry persisted in context_logs.
type ContextLog struct {
	ID               string         `json:"id"`
	ContextID        string         `json:"contextId"`
	Trigger          string         `json:"trigger"`
	Source           string         `json:"source,omitempty"`
	Model            string         `json:"model,omitempty"`
	PromptTokens     int            `json:"promptTokens,omitempty"`
	CompletionTokens int            `json:"completionTokens,omitempty"`
	DiffJSON         string         `json:"diffJson,omitempty"`
	SummaryMd        string         `json:"summaryMd"`
	IsManual         bool           `json:"isManual"`
	EditedAt         string         `json:"editedAt,omitempty"`
	CreatedAt        string         `json:"createdAt"`
	Files            []AttachedFile `json:"files,omitempty"`
}

// LogEntry is a ContextLog enriched with its context's key and organization name.
// Returned by ListAll for cross-context log views.
type LogEntry struct {
	ContextLog
	ContextKey string `json:"contextKey"`
	OrgName    string `json:"orgName,omitempty"`
}

// AttachedFile is an uploaded file referenced by a log entry.
type AttachedFile struct {
	FileID        string `json:"fileId"`
	OriginalName  string `json:"originalName"`
	FileType      string `json:"fileType"`
	ExtractedText string `json:"extractedText"`
	CreatedAt     string `json:"createdAt"`
}

// ConsolidatedLog is the rolling AI-maintained summary for a context.
type ConsolidatedLog struct {
	ContextID string `json:"contextId"`
	ContentMd string `json:"contentMd"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedAt string `json:"updatedAt"`
}

// ChatMessage is one entry in the chat transcript.
// ProposedConsolidatedMd is populated when the assistant suggests a full replacement
// of the consolidated log; AppliedAt is set when the user has accepted the proposal.
type ChatMessage struct {
	ID                     string `json:"id"`
	ContextID              string `json:"contextId"`
	Role                   string `json:"role"`
	Content                string `json:"content"`
	ProposedConsolidatedMd string `json:"proposedConsolidatedMd,omitempty"`
	AppliedAt              string `json:"appliedAt,omitempty"`
	CreatedAt              string `json:"createdAt"`
}

// ObjectSnapshot captures a single context object at a point in time.
type ObjectSnapshot struct {
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

// Snapshot is an in-memory map from objectKey (e.g. "jira_issue:JH-1") to ObjectSnapshot.
// Used to diff context state before and after a refetch.
type Snapshot map[string]ObjectSnapshot

// ObjectChange describes one diff entry between two snapshots.
// For added: Before is zero; for removed: After is zero; for changed: both are populated.
type ObjectChange struct {
	Key    string         `json:"key"`
	Before ObjectSnapshot `json:"before,omitempty"`
	After  ObjectSnapshot `json:"after,omitempty"`
}

// DiffPayload is the structured result of Diff(before, after).
type DiffPayload struct {
	Added   []ObjectChange `json:"added,omitempty"`
	Removed []ObjectChange `json:"removed,omitempty"`
	Changed []ObjectChange `json:"changed,omitempty"`
}

// IsEmpty reports whether the diff contains no added/removed/changed entries.
func (d DiffPayload) IsEmpty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}
