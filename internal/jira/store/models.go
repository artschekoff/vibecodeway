package jirastore

// JiraProvider holds Jira-specific configuration for a context.
type JiraProvider struct {
	ID          string `json:"id"`
	ContextID   string `json:"contextId"`
	SearchQuery string `json:"searchQuery"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Issue represents a Jira issue stored within a context.
type Issue struct {
	ID          string    `json:"id"`
	ContextID   string    `json:"contextId"`
	JiraKey     string    `json:"jiraKey"`
	Summary     string    `json:"summary"`
	Status      string    `json:"status"`
	Assignee    string    `json:"assignee"`
	IssueType   string    `json:"issueType"`
	Description string    `json:"description"`
	JiraUpdated string    `json:"jiraUpdated"`
	CreatedAt   string    `json:"createdAt"`
	IsActive    bool      `json:"isActive"`
	Comments    []Comment `json:"comments,omitempty"`
}

// Comment represents a Jira comment stored for an issue.
type Comment struct {
	ID          string `json:"id"`
	IssueID     string `json:"issueId"`
	JiraID      string `json:"jiraId"`
	Author      string `json:"author"`
	Body        string `json:"body"`
	JiraCreated string `json:"jiraCreated"`
	CreatedAt   string `json:"createdAt"`
	IsActive    bool   `json:"isActive"`
}
