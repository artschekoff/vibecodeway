export interface Context {
  id: string;
  key: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  issueCount?: number;
  lastItemAt?: string;
}

export interface Comment {
  id: string;
  issueId: string;
  jiraId: string;
  author: string;
  body: string;
  jiraCreated: string;
  createdAt: string;
}

export interface Issue {
  id: string;
  contextId: string;
  jiraKey: string;
  summary: string;
  status: string;
  assignee: string;
  issueType: string;
  description: string;
  jiraUpdated: string;
  createdAt: string;
  comments?: Comment[];
}

export interface JiraProvider {
  id: string;
  contextId: string;
  searchQuery: string;
  createdAt: string;
  updatedAt: string;
}

export interface ContextDetail {
  context: Context;
  jira?: JiraProvider;
  issues: Issue[];
  slackChannels?: SlackChannel[];
}

export interface SearchResult {
  key: string;
  summary: string;
  status: string;
  assignee: string;
  issuetype: string;
}

export interface SearchDiff {
  contextId: string;
  new: SearchResult[];
  existing: SearchResult[];
}

export interface SlackChannelInput {
  channelId: string;
  channelName: string;
  content?: string;
}

export interface SlackChannelGroup {
  workspace: string;
  channels: SlackChannelInput[];
}

export interface SlackSavePayload {
  groups: SlackChannelGroup[];
  removedChannelIds: string[];
}

export interface SlackChannel {
  id: string;
  contextId: string;
  workspace: string;
  channelId: string;
  channelName: string;
  createdAt: string;
}

export interface SlackChannelResult {
  id: string;
  name: string;
  content?: string;
}

export interface SlackWorkspaceResult {
  id: string;
  name: string;
}

export interface ContextFile {
  id: string;
  contextId: string;
  name: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}

export interface TextFilesPayload {
  newFiles: { name: string; content: string }[];
  editedFiles: { id: string; name: string; content: string }[];
  removedIds: string[];
}
