export interface Context {
  id: string;
  key: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  issueCount?: number;
  lastItemAt?: string;
  unviewedCount?: number;
  hasJira?: boolean;
  hasSlack?: boolean;
  hasVcs?: boolean;
  hasFiles?: boolean;
}

export interface UnviewedBreakdown {
  issues: number;
  comments: number;
  threads: number;
  dms: number;
  prs: number;
  prComments: number;
  notifications: number;
  total: number;
}

export interface Comment {
  id: string;
  issueId: string;
  jiraId: string;
  author: string;
  body: string;
  jiraCreated: string;
  createdAt: string;
  isActive: boolean;
  viewedAt?: string;
  aiProcessedAt?: string;
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
  isActive: boolean;
  viewedAt?: string;
  aiProcessedAt?: string;
  comments?: Comment[];
}

export interface JiraProvider {
  id: string;
  contextId: string;
  searchQuery: string;
  baseUrl?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ContextDetail {
  context: Context;
  jira?: JiraProvider;
  issues: Issue[];
  slackChannels?: SlackChannel[];
  vcsBranches?: VCSBranch[];
  pullRequests?: PullRequest[];
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
  startFrom?: string;
  content?: string;
}

export interface SlackChannelGroup {
  workspace: string;
  channels: SlackChannelInput[];
}

export interface SlackDMConfig {
  id: string;
  contextId: string;
  workspace: string;
  startFrom: string;
  content: string;
  createdAt: string;
  updatedAt: string;
  isActive: boolean;
  viewedAt?: string;
  aiProcessedAt?: string;
}

export interface SlackDMConfigInput {
  workspace: string;
  startFrom: string;
}

export interface SlackSavePayload {
  groups: SlackChannelGroup[];
  removedChannelIds: string[];
  dmConfigs?: SlackDMConfigInput[];
  removedDMConfigIds?: string[];
}

export interface SlackThread {
  id: string;
  contextId: string;
  slackChannelId?: string;
  workspace: string;
  channelId: string;
  threadTs: string;
  channelName: string;
  permalink: string;
  searchQuery: string;
  content: string;
  createdAt: string;
  isActive: boolean;
  viewedAt?: string;
  aiProcessedAt?: string;
}

export interface SlackChannel {
  id: string;
  contextId: string;
  workspace: string;
  channelId: string;
  channelName: string;
  startFrom: string;
  createdAt: string;
  isActive: boolean;
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

export interface ContextText {
  id: string;
  contextId: string;
  name: string;
  content: string;
  createdAt: string;
  updatedAt: string;
  isActive: boolean;
}

export interface TextFilesPayload {
  newFiles: { name: string; content: string }[];
  editedFiles: { id: string; name: string; content: string }[];
  removedIds: string[];
}

export interface UploadedFile {
  id: string;
  contextId: string;
  originalName: string;
  fileType: string;
  extractedText: string;
  fileSize: number;
  createdAt: string;
  updatedAt: string;
  isActive: boolean;
}

export interface PendingUploadPreview {
  tempId: string;
  originalName: string;
  fileType: string;
  extractedText: string;
  fileSize: number;
}

export interface UploadBatchResult {
  files: UploadedFile[];
  errors: string[];
}

export interface UploadPreviewBatchResult {
  previews: PendingUploadPreview[];
  errors: string[];
}

export interface ContextLog {
  id: string;
  contextId: string;
  trigger: string;
  source?: string;
  model?: string;
  promptTokens?: number;
  completionTokens?: number;
  diffJson?: string;
  summaryMd: string;
  isManual: boolean;
  editedAt?: string;
  createdAt: string;
}

export interface ConsolidatedLog {
  contextId: string;
  contentMd: string;
  version: number;
  updatedBy: string;
  updatedAt: string;
}

export interface ChatMessage {
  id: string;
  contextId: string;
  role: "user" | "assistant" | "system";
  content: string;
  proposedConsolidatedMd?: string;
  appliedAt?: string;
  createdAt: string;
}

export interface AISettings {
  hasKey: boolean;
  model: string;
  baseUrl: string;
  source: "env" | "db" | "none";
  updatedAt: string;
}

// --- VCS Types ---

export type VCSPlatform = "github" | "gitlab" | "linear";

export interface DeviceCodeInfo {
  userCode: string;
  verificationUri: string;
  deviceCode: string;
  expiresIn: number;
  interval: number;
}

export interface VCSPlatformInfo {
  platform: VCSPlatform;
  label: string;
  enabled: boolean;
}

export interface VCSRepo {
  id: string;
  platform: VCSPlatform;
  owner: string;
  name: string;
  createdAt: string;
}

export interface VCSBranch {
  id: string;
  contextId: string;
  repoId: string;
  branch: string;
  createdAt: string;
  repo?: VCSRepo;
}

export interface PullRequest {
  id: string;
  repoId: string;
  branchId: string;
  prNumber: number;
  title: string;
  author: string;
  state: "open" | "closed" | "merged" | string;
  reviewStatus: "approved" | "changes_requested" | "review_required" | "commented" | string;
  /** GitHub mergeability: "MERGEABLE" | "CONFLICTING" | "UNKNOWN" | "" */
  mergeable?: string;
  url: string;
  headBranch?: string;
  prCreatedAt: string;
  prUpdatedAt: string;
  fetchedAt: string;
  viewedAt?: string;
  aiProcessedAt?: string;
  comments?: PRComment[];
}

export interface PRComment {
  id: string;
  prId: string;
  externalId: string;
  parentId?: string;
  commentType: "conversation" | "review" | "review_comment" | string;
  author: string;
  body: string;
  path?: string;
  line?: number;
  isResolved: boolean;
  inReplyToId?: string;
  commentCreatedAt: string;
  commentUpdatedAt: string;
  viewedAt?: string;
  aiProcessedAt?: string;
  replies?: PRComment[];
}

export interface VCSAuth {
  id: string;
  platform: VCSPlatform;
  username: string;
  avatarUrl: string;
  updatedAt: string;
}

export interface ReviewRequest {
  id: string;
  repoId: string;
  prNumber: number;
  prTitle: string;
  prAuthor: string;
  prUrl: string;
  reviewStatus: string;
  requestedAt: string;
  fetchedAt: string;
}
