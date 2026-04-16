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
	LastItemAt string `json:"lastItemAt,omitempty"`
}

// ContextFile is a plain-text document associated with a context.
// Multiple files can be attached to a single context (1 context : many files).
type ContextFile struct {
	ID        string `json:"id"`
	ContextID string `json:"contextId"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	IsActive  bool   `json:"isActive"`
}
