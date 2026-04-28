package store

// Context is a named bucket that aggregates data from multiple providers.
// Provider-specific fields live in their respective provider tables.
type Context struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	Status     string `json:"status"`
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

// AISettings holds OpenAI configuration stored in the local database.
// The raw API key is never persisted or serialised — only the flag HasKey is exposed.
type AISettings struct {
	HasKey    bool   `json:"hasKey"`
	Model     string `json:"model"`
	BaseURL   string `json:"baseUrl"`
	UpdatedAt string `json:"updatedAt"`
}
