export interface Context {
  id: string;
  key: string;
  status: string;
  orgId: string;
  createdAt: string;
  updatedAt: string;
  issueCount?: number;
  lastItemAt?: string;
  unviewedCount?: number;
  hasJira?: boolean;
  hasSlack?: boolean;
  hasVcs?: boolean;
  hasFiles?: boolean;
  // Snapshot fields
  snapshotStatus: string;      // from context_snapshots.status
  snapshotIsStale: boolean;    // true when new data needs re-analysis
}

// Derived helper
export function effectiveStatus(c: Context): string {
  return c.snapshotStatus || "not_started";
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
  parentKey: string;
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
  relations?: ContextRelation[];
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

export interface SlackMessageItem {
  user: string;
  userId: string;
  text: string;
  timestamp: string;
  rawTs?: string;
  replies?: SlackMessageItem[];
}

export interface SlackChannelInput {
  channelId: string;
  channelName: string;
  startFrom?: string;
  messages?: SlackMessageItem[];
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
  parentThreadTs?: string;
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
  messages?: SlackMessageItem[];
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

export interface AppLogEntry {
  id: number;
  level: string;
  ts: number;
  caller: string;
  msg: string;
  error?: string;
  createdAt: string;
}

export interface ContextSnapshot {
  contextId: string;
  currentStepDesc: string;
  currentStepConfidence: "high" | "medium" | "low";
  currentStepFrom: string;   // JSON array string
  currentStepOverridden: boolean;
  sectionOperational: string;
  sectionNarrative: string;
  sectionEngineering: string;
  status: string;
  aiNextStepsShort: string;
  aiNextStepsMid: string;
  aiNextStepsLong: string;
  userNextStepsShort: string;
  userNextStepsMid: string;
  userNextStepsLong: string;
  lastAnalyzedSessionId?: string;
  analyzedAt?: string;
  isStale: boolean;
  version: number;
  estimatedHours?: number;
  estimatedCompletionDate?: string;
  estimationConfidence: string;   // "high" | "medium" | "low" | ""
  estimationBasis: string;
}

export interface EstimationLog {
  id: string;
  contextId: string;
  recordedAt: string;
  triggerType: "initial" | "re-analysis" | "requirement_change" | string;
  triggerDescription: string;
  deltaHours?: number;
  estimatedHoursBefore?: number;
  estimatedHoursAfter?: number;
  estimatedCompletionBefore?: string;
  estimatedCompletionAfter?: string;
  estimationBasis: string;
}

export interface TaskAssignment {
  contextId: string;
  content: string;
  generatedAt?: string;
  updatedAt: string;
}

export interface Keybinding {
  id: string;
  command: string;
  label: string;
  description: string;
  modifierKeys: string;  // comma-separated: "cmd,shift"
  keyCode: string;       // "c", "enter", "space", etc.
  updatedAt: string;
}

export interface ContextEvent {
  id: string;
  contextId: string;
  syncSessionId?: string;
  type: string;
  timestamp: string;
  sourceType: "jira" | "slack" | "github" | "chat" | "user";
  sourceId: string;
  author: string;
  summary: string;
  rawReference: string;
  createdAt: string;
}

export interface AISettings {
  provider: "openai" | "anthropic" | "local";
  hasKey: boolean;
  hasOpenAIKey: boolean;
  hasAnthropicKey: boolean;
  model: string;
  baseUrl: string;
  source: "env" | "db" | "none";
  updatedAt: string;
}

export interface AppSettings {
  contextRefetchEnabled: boolean;
  contextRefetchIntervalMin: number;
  reviewsRefetchEnabled: boolean;
  reviewsRefetchIntervalMin: number;
  terminalAppPath: string;
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
  platform: string;
  owner: string;
  name: string;
  localPath: string;
  label: string;
  description: string;
  onCheckoutCommand: string;
  createdAt: string;
}

export interface VCSRepoPath {
  id: string;
  repoId: string;
  path: string;
  createdAt: string;
}

export interface LocalBranch {
  name: string;
  isCurrent: boolean;
  isGone: boolean;
}

export interface VCSWorkspace {
  id: string;
  repoPathId: string;
  repoId: string;
  branch: string;
  worktreePath: string;
  status: "active" | "should_delete";
  createdAt: string;
  updatedAt: string;
}

export interface VCSBranch {
  id: string;
  contextId: string;
  repoId: string;
  branch: string;
  label: string;
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
  /** Aggregated CI status: "PENDING" | "SUCCESS" | "FAILURE" | "" */
  ciStatus?: string;
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
  headBranch?: string;
  requestedAt: string;
  fetchedAt: string;
  reviewedAt?: string;
  dismissedAt?: string;
}

// --- Context Relations ---

export type ContextRelationType =
  | "blocks"
  | "is-blocked-by"
  | "relates-to"
  | "duplicates"
  | "is-duplicated-by";

export const RELATION_TYPE_LABELS: Record<ContextRelationType, string> = {
  "blocks": "Blocks",
  "is-blocked-by": "Is Blocked By",
  "relates-to": "Relates To",
  "duplicates": "Duplicates",
  "is-duplicated-by": "Is Duplicated By",
};

export interface ContextRelation {
  id: string;
  contextId: string;
  targetId: string;
  targetKey: string;
  relationType: ContextRelationType;
  createdAt: string;
}

export interface RelationInput {
  targetId: string;
  relationType: ContextRelationType;
}

export interface RelationsPayload {
  newRelations: RelationInput[];
  removedRelationIds: string[];
}

export interface WorkflowStatus {
  id: string;
  slug: string;
  label: string;
  isSystem: boolean;
  createdAt: string;
}

export type WorkflowStatusValue =
  | 'not_started'
  | 'scoping'
  | 'in_progress'
  | 'needs_attention'
  | 'ready';

// --- Organization ---

export interface Organization {
  id: string;
  name: string;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
}

// --- Tab system ---

export type TabType = "org" | "context" | "settings" | "logs" | "prompts";

export interface Tab {
  id: string;          // unique tab instance ID (not entity ID)
  type: TabType;
  orgId?: string;      // present when type === "org"; also on type === "context" to link back to parent org
  orgName?: string;    // org display name; stored on context tabs for fallback org-tab creation
  contextId?: string;  // present when type === "context"
  contextKey?: string; // raw context key without org prefix; used for display in TabItem
  title: string;
  pinned: boolean;
}
