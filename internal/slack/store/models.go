package slackstore

// SlackProvider holds Slack-specific configuration for a context.
type SlackProvider struct {
	ID          string `json:"id"`
	ContextID   string `json:"contextId"`
	Workspace   string `json:"workspace"`
	SearchQuery string `json:"searchQuery"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// SlackThread represents a loaded Slack thread stored within a context.
type SlackThread struct {
	ID             string `json:"id"`
	ContextID      string `json:"contextId"`
	SlackChannelID string `json:"slackChannelId,omitempty"`
	Workspace      string `json:"workspace"`
	ChannelID      string `json:"channelId"`
	ThreadTS       string `json:"threadTs"`
	ChannelName    string `json:"channelName"`
	Permalink      string `json:"permalink"`
	SearchQuery    string `json:"searchQuery"`
	Content        string `json:"content"`
	CreatedAt      string `json:"createdAt"`
	IsActive       bool   `json:"isActive"`
}

// UpsertSlackThreadArgs holds all fields for UpsertSlackThread.
type UpsertSlackThreadArgs struct {
	SlackChannelID string // optional FK to slack_channels.id; enables ON DELETE CASCADE
	Workspace      string
	ChannelID      string
	ThreadTS       string
	ChannelName    string
	Permalink      string
	SearchQuery    string
	Content        string
}

// SlackChannel represents a Slack channel configured for a context.
type SlackChannel struct {
	ID          string `json:"id"`
	ContextID   string `json:"contextId"`
	Workspace   string `json:"workspace"`
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	StartFrom   string `json:"startFrom"` // YYYY-MM-DD; passed as --start-from on refresh
	CreatedAt   string `json:"createdAt"`
	IsActive    bool   `json:"isActive"`
}

// SlackChannelInput carries the fields needed to add a channel to a context.
// When Content is set (populated from the search-channels CLI response), AddSlackChannels
// stores it immediately as a slack_thread record so the user sees content without a Refresh.
type SlackChannelInput struct {
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	StartFrom   string `json:"startFrom,omitempty"` // YYYY-MM-DD; optional
	Content     string `json:"content,omitempty"`
}

// SlackDMConfig holds configuration for loading personal (direct) messages for a context.
// One row per (context, workspace): stores the start_from date passed to slack-cli list-dms.
type SlackDMConfig struct {
	ID        string `json:"id"`
	ContextID string `json:"contextId"`
	Workspace string `json:"workspace"`
	StartFrom string `json:"startFrom"` // YYYY-MM-DD
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	IsActive  bool   `json:"isActive"`
}
