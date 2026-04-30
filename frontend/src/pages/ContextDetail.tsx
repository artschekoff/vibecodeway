import { useState, useEffect, useCallback, useRef } from "react";
import Spinner from "../components/Spinner";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Check, CheckCheck, ChevronDown, Copy, ExternalLink, GitBranch, Pencil, RefreshCw, X } from "lucide-react";
import {
  GetContextDetail,
  RefreshContext,
  CancelRefresh,
  DeleteJiraIssue,
  DeleteJiraComment,
  DeleteSlackThread,
  RemoveSlackChannel,
  DeleteContextText,
  DeleteUploadedFile,
  ToggleIssueActive,
  ToggleCommentActive,
  ToggleSlackChannelActive,
  ToggleSlackThreadActive,
  ToggleContextTextActive,
  ToggleUploadedFileActive,
  ToggleSlackDMConfigActive,
  UpdateContext,
  SetJiraConfig,
  AddIssuesToContext,
  AddSlackChannels,
  UpsertContextText,
  UpsertSlackDMConfig,
  DeleteSlackDMConfig,
  RemoveSlackDMEntry,
  ListSlackDMConfigs,
  AddVCSBranch,
  RemoveVCSBranch,
  // @ts-ignore
  AddContextRelation,
  // @ts-ignore
  RemoveContextRelation,
  GetUnviewedBreakdown,
  MarkContextAllViewed,
  MarkIssueViewed,
  MarkCommentViewed,
  MarkSlackThreadViewed,
  MarkSlackDMConfigViewed,
  MarkPRViewed,
  MarkPRCommentViewed,
  GetWorkflowSummary,
  GetOpenQuestions,
  GetWorkflowProposals,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL, EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { service } from "../../wailsjs/go/models";
import type { Context, Issue, SlackChannel, ContextText, SlackDMConfig, SlackSavePayload, TextFilesPayload, UploadedFile, VCSBranch, PullRequest, UnviewedBreakdown, ContextRelation, RelationsPayload, WorkflowSummary, OpenQuestion, WorkflowProposal } from "../types";
import { RELATION_TYPE_LABELS } from "../types";
import type { VCSBranchInput } from "../components/ContextForm";
import IssueCard from "../components/IssueCard";
import SlackChannelCard from "../components/SlackChannelCard";
import SlackDMConfigCard from "../components/SlackDMConfigCard";
import ContextChatPanel from "../components/ContextChatPanel";
import ContextLogList from "../components/ContextLogList";
import ConsolidatedLogView from "../components/ConsolidatedLogView";
import UploadedFileCard from "../components/UploadedFileCard";
import ContextTextCard from "../components/ContextTextCard";
import ContextForm from "../components/ContextForm";
import WorkflowProposalCard from "../components/WorkflowProposalCard";
import OpenQuestionsPanel from "../components/OpenQuestionsPanel";
import { useProgress } from "../contexts/ProgressContext";

function PRStatusBadge({ status, reviewStatus, mergeable, ciStatus }: { status: string; reviewStatus: string; mergeable?: string; ciStatus?: string }) {
  const isMerged = status === "merged";
  const isClosed = status === "closed";

  const reviewLabel = !isMerged && !isClosed
    ? (reviewStatus === "changes_requested" ? "Changes Requested"
      : reviewStatus === "approved" ? "Approved"
      : reviewStatus === "review_required" ? "Review Required"
      : reviewStatus || null)
    : null;

  const isReadyToMerge = !isMerged && !isClosed
    && ciStatus === "SUCCESS" && reviewStatus === "approved" && mergeable === "MERGEABLE";

  return (
    <div className="pr-badges">
      <span className={`pr-status-badge pr-status-badge--${status}`}>{status}</span>
      {reviewLabel && (
        <span className={`pr-status-badge pr-review-badge${reviewStatus === "changes_requested" ? " pr-status--changes-requested" : reviewStatus === "approved" ? " pr-status--approved" : ""}`}>
          {reviewLabel}
        </span>
      )}
      {!isMerged && !isClosed && mergeable === "CONFLICTING" && (
        <span className="pr-status-badge pr-status-badge--conflict" title="This PR has merge conflicts">
          Conflicts
        </span>
      )}
      {!isMerged && !isClosed && ciStatus === "PENDING" && (
        <span className="pr-status-badge pr-status-badge--ci-pending" title="CI checks running">
          CI running
        </span>
      )}
      {!isMerged && !isClosed && ciStatus === "FAILURE" && (
        <span className="pr-status-badge pr-status-badge--ci-failure" title="CI checks failed">
          CI failed
        </span>
      )}
      {isReadyToMerge && (
        <span className="pr-status-badge pr-status-badge--ready" title="All checks passed and approved — ready to merge">
          Ready to merge
        </span>
      )}
    </div>
  );
}

function PRCommentItem({ comment, depth = 0, onMarkViewed }: { comment: import("../types").PRComment; depth?: number; onMarkViewed?: (id: string) => void }) {
  const isUnread = !comment.viewedAt;
  return (
    <div className={`pr-comment${comment.isResolved ? " pr-comment--resolved" : ""}${isUnread ? " item--unread" : ""}`} style={{ marginLeft: depth * 12 }}>
      <div className="pr-comment-meta">
        <span className="pr-comment-author">{comment.author || "Unknown"}</span>
        {comment.path && <span className="pr-comment-path">{comment.path}{comment.line ? `:${comment.line}` : ""}</span>}
        {comment.isResolved && <span className="pr-comment-resolved-badge">resolved</span>}
        {isUnread && onMarkViewed && (
          <button className="mark-viewed-btn" onClick={() => onMarkViewed(comment.id)} title="Mark as viewed"><Check size={13} /></button>
        )}
      </div>
      <div className="pr-comment-body">{comment.body}</div>
      {(comment.replies ?? []).map((r) => (
        <PRCommentItem key={r.id} comment={r} depth={depth + 1} onMarkViewed={onMarkViewed} />
      ))}
    </div>
  );
}

function PRCard({ pr, vcsBranches, onMarkPRViewed, onMarkCommentViewed }: {
  pr: PullRequest;
  vcsBranches?: VCSBranch[];
  onMarkPRViewed?: (id: string) => void;
  onMarkCommentViewed?: (id: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleCopyUrl = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!pr.url) return;
    void navigator.clipboard.writeText(pr.url).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };
  const unresolvedComments = (pr.comments ?? []).filter((c) => !c.isResolved);
  const isUnread = !pr.viewedAt;
  const hasUnreadComments = unresolvedComments.some((c) => !c.viewedAt);
  const linkedBranch = vcsBranches?.find((b) => b.id === pr.branchId);
  const repoBase = linkedBranch?.repo
    ? `https://github.com/${linkedBranch.repo.owner}/${linkedBranch.repo.name}`
    : null;
  const headBranchLink = repoBase && pr.headBranch
    ? `${repoBase}/tree/${pr.headBranch}`
    : null;

  return (
    <div className={`pr-card${isUnread || hasUnreadComments ? " item--unread-parent" : ""}${isUnread ? " pr-card--unread" : ""}`}>
      {/* Top row: number + title + action buttons */}
      <div className="pr-card-top" onClick={() => setExpanded((v) => !v)}>
        <div className="pr-card-title-row">
          <span className="pr-number">#{pr.prNumber}</span>
          <span className="pr-title">{pr.title}</span>
        </div>
        <div className="pr-card-actions">
          {pr.url && (
            <button
              type="button"
              className="pr-action-btn"
              onClick={handleCopyUrl}
              title="Copy PR link"
            >{copied ? <Check size={13} /> : <Copy size={13} />}</button>
          )}
          {pr.url && (
            <button
              type="button"
              className="pr-action-btn"
              onClick={(e) => { e.stopPropagation(); BrowserOpenURL(pr.url!); }}
              title="Open on GitHub"
            ><ExternalLink size={13} /></button>
          )}
          {isUnread && onMarkPRViewed && (
            <button
              type="button"
              className="pr-action-btn pr-action-btn--viewed"
              onClick={(e) => { e.stopPropagation(); onMarkPRViewed(pr.id); }}
              title="Mark as viewed"
            ><Check size={13} /></button>
          )}
          <ChevronDown size={15} className={`pr-expand-chevron${expanded ? " pr-expand-chevron--open" : ""}`} />
        </div>
      </div>

      {/* Bottom row: author + branch + status badges */}
      <div className="pr-card-meta">
        <span className="pr-author">@{pr.author}</span>
        {pr.headBranch && (
          headBranchLink ? (
            <button
              type="button"
              className="pr-branch-chip pr-branch-chip--link"
              onClick={(e) => { e.stopPropagation(); BrowserOpenURL(headBranchLink); }}
              title={`Open branch ${pr.headBranch} on GitHub`}
            >
              <GitBranch size={11} /> {pr.headBranch}
            </button>
          ) : (
            <span className="pr-branch-chip"><GitBranch size={11} /> {pr.headBranch}</span>
          )
        )}
        <PRStatusBadge status={pr.state} reviewStatus={pr.reviewStatus} mergeable={pr.mergeable} ciStatus={pr.ciStatus} />
      </div>

      {expanded && (
        <div className="pr-comment-tree">
          {unresolvedComments.length === 0 ? (
            <div style={{ fontSize: 12, color: "var(--text-muted)", padding: "8px 0" }}>No unresolved comments.</div>
          ) : (
            unresolvedComments.map((c) => <PRCommentItem key={c.id} comment={c} onMarkViewed={onMarkCommentViewed} />)
          )}
        </div>
      )}
    </div>
  );
}

export default function ContextDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const progress = useProgress();
  const [detail, setDetail] = useState<service.ContextDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshError, setRefreshError] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [toastMsg, setToastMsg] = useState("");
  const [toastType, setToastType] = useState<"success" | "error">("success");
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [sidePanelTab, setSidePanelTab] = useState<"chat" | "logs" | "consolidated">("chat");
  const [consolidatedReloadKey, setConsolidatedReloadKey] = useState(0);
  const [editOpen, setEditOpen] = useState(false);
  const [editJqlQuery, setEditJqlQuery] = useState<string | undefined>(undefined);
  const [editJiraIssues, setEditJiraIssues] = useState<Issue[] | undefined>(undefined);
  const [editSlackChannels, setEditSlackChannels] = useState<SlackChannel[] | undefined>(undefined);
  const [editSlackDMConfigs, setEditSlackDMConfigs] = useState<SlackDMConfig[] | undefined>(undefined);
  const [editContextTexts, setEditContextTexts] = useState<ContextText[] | undefined>(undefined);
  const [editUploadedFiles, setEditUploadedFiles] = useState<UploadedFile[] | undefined>(undefined);
  const [editVCSBranches, setEditVCSBranches] = useState<VCSBranch[] | undefined>(undefined);
  const [editRelations, setEditRelations] = useState<ContextRelation[] | undefined>(undefined);
  const [breakdown, setBreakdown] = useState<UnviewedBreakdown | null>(null);
  const [markingAllViewed, setMarkingAllViewed] = useState(false);
  const [workflowSummary, setWorkflowSummary] = useState<WorkflowSummary | null>(null);
  const [openQuestions, setOpenQuestions] = useState<OpenQuestion[]>([]);
  const [proposals, setProposals] = useState<WorkflowProposal[]>([]);
  const [showProposalHistory, setShowProposalHistory] = useState(false);
  const [workflowError, setWorkflowError] = useState("");

  const loadBreakdown = useCallback(async () => {
    if (!id) return;
    try {
      const bd = await GetUnviewedBreakdown(id);
      setBreakdown(bd);
    } catch {
      // best-effort
    }
  }, [id]);

  const loadWorkflow = useCallback(async () => {
    if (!id) return;
    setWorkflowError("");
    try {
      const [summary, questions, props] = await Promise.all([
        GetWorkflowSummary(id) as Promise<WorkflowSummary>,
        GetOpenQuestions(id) as Promise<OpenQuestion[]>,
        GetWorkflowProposals(id) as Promise<WorkflowProposal[]>,
      ]);
      setWorkflowSummary(summary);
      setOpenQuestions(questions ?? []);
      setProposals(props ?? []);
    } catch {
      setWorkflowError("Failed to load workflow data");
    }
  }, [id]);

  const load = useCallback(async (silent = false) => {
    if (!id) return;
    if (!silent) setLoading(true);
    try {
      const d = await GetContextDetail(id);
      setDetail(d);
      void loadBreakdown();
    } catch {
      // Wails bridge not available or load failed — fail silently
    } finally {
      if (!silent) setLoading(false);
    }
  }, [id, loadBreakdown]);

  const handleMarkAllViewed = async () => {
    if (!id) return;
    setMarkingAllViewed(true);
    try {
      await MarkContextAllViewed(id);
      await loadBreakdown();
      load(true);
    } catch {
      // best-effort
    } finally {
      setMarkingAllViewed(false);
    }
  };

  useEffect(() => {
    const onUpdated = (e: Event) => {
      const ev = e as CustomEvent<{ id: string }>;
      if (ev.detail?.id === id) load(true);
    };
    window.addEventListener("context:updated", onUpdated);
    return () => window.removeEventListener("context:updated", onUpdated);
  }, [id, load]);

  useEffect(() => {
    load();
    void loadWorkflow();
  }, [load, loadWorkflow]);

  const handleRefresh = async () => {
    if (!id) return;
    setRefreshing(true);
    setRefreshError("");
    setToastMsg("");

    progress.begin("Refreshing Context", [], () => { void CancelRefresh(); });

    const REFRESH_LABELS: Record<string, string> = {
      jira: "Syncing Jira issues & comments",
      slack: "Refreshing Slack threads & DMs",
      vcs: "Fetching pull requests",
      text: "Syncing texts & files",
    };

    const unsubscribe = EventsOn(
      "refresh:progress",
      (data: { current: number; total: number; step: string }) => {
        const label = REFRESH_LABELS[data.step] ?? `Syncing ${data.step}`;
        progress.set(data.current - 1, data.total, label);
      }
    );

    try {
      const result = await RefreshContext(id);
      const parts: string[] = [];
      if (result.issuesRefreshed > 0) parts.push(`${result.issuesRefreshed} issue${result.issuesRefreshed !== 1 ? "s" : ""} updated`);
      if (result.newComments > 0) parts.push(`${result.newComments} new comment${result.newComments !== 1 ? "s" : ""}`);
      if (result.threadsRefreshed > 0) parts.push(`${result.threadsRefreshed} Slack thread${result.threadsRefreshed !== 1 ? "s" : ""} refreshed`);
      if (result.dmsRefreshed > 0) parts.push(`${result.dmsRefreshed} DM${result.dmsRefreshed !== 1 ? "s" : ""} refreshed`);
      if (result.prsRefreshed > 0) parts.push(`${result.prsRefreshed} PR${result.prsRefreshed !== 1 ? "s" : ""} synced`);
      if (result.newPRComments > 0) parts.push(`${result.newPRComments} PR comment${result.newPRComments !== 1 ? "s" : ""}`);
      if (result.newNotifications > 0) parts.push(`${result.newNotifications} notification${result.newNotifications !== 1 ? "s" : ""}`);
      const msg = parts.length > 0 ? parts.join(" · ") : "Everything is up to date.";
      showToast(msg);
      load(true);
      void loadWorkflow();
    } catch (e) {
      setRefreshError(String(e));
    } finally {
      unsubscribe();
      EventsOff("refresh:progress");
      setRefreshing(false);
      progress.done();
    }
  };

  const openEditForm = async () => {
    if (!id) return;
    let jql = "";
    let issues: Issue[] = [];
    let channels: SlackChannel[] = [];
    let dmConfigs: SlackDMConfig[] = [];
    let contextTexts: ContextText[] = [];
    try {
      const [det, dms] = await Promise.all([
        GetContextDetail(id),
        ListSlackDMConfigs(id),
      ]);
      jql = det.jira?.searchQuery ?? "";
      issues = det.issues ?? [];
      channels = det.slackChannels ?? [];
      contextTexts = det.contextTexts ?? [];
      dmConfigs = dms ?? [];
    } catch {
      // best-effort — open with empty values
    }
    setEditJqlQuery(jql);
    setEditJiraIssues(issues);
    setEditSlackChannels(channels);
    setEditSlackDMConfigs(dmConfigs);
    setEditContextTexts(contextTexts);
    setEditUploadedFiles((detail?.uploadedFiles ?? []) as UploadedFile[]);
    setEditVCSBranches((detail?.vcsBranches ?? []) as VCSBranch[]);
    setEditRelations((detail?.relations ?? []) as ContextRelation[]);
    setEditOpen(true);
  };

  const closeEditForm = () => {
    setEditOpen(false);
    setEditJqlQuery(undefined);
    setEditJiraIssues(undefined);
    setEditSlackChannels(undefined);
    setEditSlackDMConfigs(undefined);
    setEditContextTexts(undefined);
    setEditUploadedFiles(undefined);
    setEditVCSBranches(undefined);
    setEditRelations(undefined);
  };

  const handleUpdate = async (
    key: string,
    searchQuery: string,
    status: string | undefined,
    slackPayload?: SlackSavePayload,
    selectedJiraKeys?: string[],
    _removedJiraIssueIds?: string[],
    textFilesPayload?: TextFilesPayload,
    removedUploadedFileIds?: string[],
    _pendingUploadTempIds?: string[],
    newVCSBranches?: VCSBranchInput[],
    removedVCSBranchIds?: string[],
    relationsPayload?: RelationsPayload
  ) => {
    if (!id || !detail) return;
    const jiraKeys = selectedJiraKeys ?? [];
    const slackGroups = (slackPayload?.groups ?? []).filter((g) => g.channels.length > 0);
    const removedChannelIds = slackPayload?.removedChannelIds ?? [];
    const dmConfigs = slackPayload?.dmConfigs ?? [];
    const removedDMConfigIds = slackPayload?.removedDMConfigIds ?? [];
    const newTextFiles = textFilesPayload?.newFiles ?? [];
    const editedTextFiles = textFilesPayload?.editedFiles ?? [];
    const removedTextFileIds = textFilesPayload?.removedIds ?? [];
    const itemLabels = [
      ...jiraKeys,
      ...slackGroups.flatMap((g) =>
        g.channels.map((ch) => `#${ch.channelName}  ·  ${g.workspace}`)
      ),
      ...dmConfigs.map((dm) => `DMs from ${dm.workspace}`),
      ...newTextFiles.map((f) => f.name || "Text file"),
    ];

    progress.begin("Updating Context", itemLabels);

    try {
      await UpdateContext(id, key, status ?? detail.context.status);
      if (searchQuery.trim()) {
        await SetJiraConfig(id, searchQuery.trim());
      }
      for (const jiraKey of jiraKeys) {
        progress.next(`Adding issue ${jiraKey}`);
        try { await AddIssuesToContext(id, [jiraKey]); } catch { /* best-effort */ }
      }
      for (const chanId of removedChannelIds) {
        try { await RemoveSlackChannel(chanId); } catch { /* best-effort */ }
      }
      for (const group of slackGroups) {
        const names = group.channels.map((ch) => `#${ch.channelName}`).join(", ");
        progress.next(`Adding ${names} from ${group.workspace}`);
        try { await AddSlackChannels(id, group.workspace, group.channels); } catch { /* best-effort */ }
      }
      for (const dmId of removedDMConfigIds) {
        try { await DeleteSlackDMConfig(dmId); } catch { /* best-effort */ }
      }
      for (const dm of dmConfigs) {
        progress.next(`Loading DMs from ${dm.workspace}`);
        try { await UpsertSlackDMConfig(id, dm.workspace, dm.startFrom); } catch { /* best-effort */ }
      }
      for (const fId of removedTextFileIds) {
        try { await DeleteContextText(fId); } catch { /* best-effort */ }
      }
      for (const f of editedTextFiles) {
        try { await UpsertContextText(f.id, id, f.name, f.content); } catch { /* best-effort */ }
      }
      for (const f of newTextFiles) {
        progress.next(`Saving ${f.name || "text file"}`);
        try { await UpsertContextText("", id, f.name, f.content); } catch { /* best-effort */ }
      }
      // Uploaded file removals are already applied eagerly inside ContextForm;
      // call again as safety net in case the form state diverged.
      for (const fId of (removedUploadedFileIds ?? [])) {
        try { await DeleteUploadedFile(fId); } catch { /* best-effort */ }
      }
      // VCS branches
      for (const branchId of (removedVCSBranchIds ?? [])) {
        try { await RemoveVCSBranch(branchId); } catch { /* best-effort */ }
      }
      for (const b of (newVCSBranches ?? [])) {
        progress.next(`Linking ${b.ownerRepo}:${b.branch}`);
        try { await AddVCSBranch(id, b.platform, b.ownerRepo, b.branch, b.label ?? ""); } catch { /* best-effort */ }
      }
      // Relations
      for (const relId of (relationsPayload?.removedRelationIds ?? [])) {
        try { await RemoveContextRelation(relId); } catch { /* best-effort */ }
      }
      for (const rel of (relationsPayload?.newRelations ?? [])) {
        try { await AddContextRelation(id, rel.targetId, rel.relationType); } catch { /* best-effort */ }
      }
    } finally {
      progress.done();
    }

    closeEditForm();
    load(true);
  };

  const showToast = (msg: string, type: "success" | "error" = "success") => {
    setToastMsg(msg);
    setToastType(type);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToastMsg(""), type === "error" ? 5000 : 3000);
  };

  const handleDeleteIssue = async (issueId: string) => {
    try {
      await DeleteJiraIssue(issueId);
      showToast("Issue removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteComment = async (commentId: string) => {
    try {
      await DeleteJiraComment(commentId);
      showToast("Comment removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteChannel = async (channelId: string) => {
    try {
      await RemoveSlackChannel(channelId);
      showToast("Channel removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteThread = async (threadId: string) => {
    try {
      await DeleteSlackThread(threadId);
      showToast("Thread removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteContextText = async (textId: string) => {
    try {
      await DeleteContextText(textId);
      showToast("Text removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteUploadedFile = async (fileId: string) => {
    try {
      await DeleteUploadedFile(fileId);
      showToast("File removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleUploadedFile = async (fileId: string) => {
    try {
      await ToggleUploadedFileActive(fileId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleIssue = async (issueId: string) => {
    try {
      await ToggleIssueActive(issueId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleComment = async (commentId: string) => {
    try {
      await ToggleCommentActive(commentId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleChannel = async (channelId: string) => {
    try {
      await ToggleSlackChannelActive(channelId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleThread = async (threadId: string) => {
    try {
      await ToggleSlackThreadActive(threadId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleContextText = async (textId: string) => {
    try {
      await ToggleContextTextActive(textId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteDMConfig = async (dmId: string) => {
    try {
      await DeleteSlackDMConfig(dmId);
      showToast("DM config removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleToggleDMConfig = async (dmId: string) => {
    try {
      await ToggleSlackDMConfigActive(dmId);
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleDeleteDMEntry = async (dmConfigId: string, entryId: string) => {
    try {
      await RemoveSlackDMEntry(dmConfigId, entryId);
      showToast("DM conversation removed");
      load(true);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  if (loading) return <div className="loading">Loading…</div>;

  if (!detail) return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => navigate("/")}><ArrowLeft size={16} /> Back</button>
      </div>
      <div className="content">
        <div className="error-banner">{refreshError || "Context not found"}</div>
      </div>
    </div>
  );

  const { context } = detail;
  const issues = detail.issues ?? [];
  const slackChannels = detail.slackChannels ?? [];
  const slackThreads = detail.slackThreads ?? [];
  const slackDMConfigs = detail.slackDMConfigs ?? [];
  const contextTexts = detail.contextTexts ?? [];
  const uploadedFiles = (detail.uploadedFiles ?? []) as UploadedFile[];
  const vcsBranches = (detail.vcsBranches ?? []) as VCSBranch[];
  const pullRequests = (detail.pullRequests ?? []) as PullRequest[];
  const isEmpty = issues.length === 0 && slackChannels.length === 0 && slackThreads.length === 0 && slackDMConfigs.length === 0 && contextTexts.length === 0 && uploadedFiles.length === 0 && vcsBranches.length === 0;

  return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => navigate("/")}><ArrowLeft size={16} /> Back</button>
        <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
          <h1 style={{ fontSize: 16, margin: 0, whiteSpace: "nowrap" }}>{context.key}</h1>
          <span className={`badge badge-${context.status}`}>{context.status}</span>
          <span className="topbar-counts">
            {[
              issues.length > 0 && `${issues.length} issue${issues.length !== 1 ? "s" : ""}`,
              slackChannels.length > 0 && `${slackChannels.length} channel${slackChannels.length !== 1 ? "s" : ""}`,
              slackThreads.length > 0 && `${slackThreads.length} thread${slackThreads.length !== 1 ? "s" : ""}`,
              slackDMConfigs.length > 0 && `${slackDMConfigs.length} DM${slackDMConfigs.length !== 1 ? "s" : ""}`,
              (contextTexts.length + uploadedFiles.length) > 0 && `${contextTexts.length + uploadedFiles.length} file${(contextTexts.length + uploadedFiles.length) !== 1 ? "s" : ""}`,
            ].filter(Boolean).join(" · ")}
          </span>
        </div>
        <div className="topbar-actions">
          {breakdown && breakdown.total > 0 && (
            <button
              className="btn-secondary"
              onClick={() => void handleMarkAllViewed()}
              disabled={markingAllViewed}
              title="Mark all items as viewed"
            >
              {markingAllViewed ? "Marking…" : <><CheckCheck size={16} /> Mark all viewed ({breakdown.total})</>}
            </button>
          )}
          <button
            className="btn-secondary"
            onClick={() => void openEditForm()}
            title="Edit context"
          >
            <Pencil size={16} /> Edit
          </button>
          <button
            className="btn-primary"
            onClick={() => void handleRefresh()}
            disabled={refreshing}
          >
            {refreshing ? (
              <span className="btn-with-spinner">
                <Spinner size={14} color="#fff" />
                Refreshing…
              </span>
            ) : (
              <><RefreshCw size={16} /> Refresh</>
            )}
          </button>
        </div>
      </div>

      <div className="content content--split">
        {refreshError && <div className="error-banner">{refreshError}</div>}
        {toastMsg && <div className={`toast toast--${toastType}`}>{toastMsg}</div>}

        <div className="detail-primary">
        {/* Workflow status + open questions */}
        {workflowError && <div className="error-banner" style={{ marginBottom: 8 }}>{workflowError}</div>}
        {workflowSummary && (
          <WorkflowProposalCard
            contextId={id!}
            summary={workflowSummary}
            onUpdated={(updated) => setWorkflowSummary(updated)}
          />
        )}
        <OpenQuestionsPanel questions={openQuestions} onUpdated={() => void loadWorkflow()} />

        {/* Proposal history */}
        {proposals.length > 0 && (
          <div className="workflow-history">
            <button
              className="workflow-history-toggle"
              onClick={() => setShowProposalHistory((v) => !v)}
            >
              {showProposalHistory ? "▾" : "▸"} Proposal History{proposals.length > 5 ? ` (last 5 of ${proposals.length})` : ` (${proposals.length})`}
            </button>
            {showProposalHistory && (
              <div className="workflow-history-list">
                {proposals.slice(0, 5).map((p) => (
                  <div key={p.id} className="workflow-history-item">
                    <span className="workflow-history-date">
                      {(() => { try { return new Date(p.createdAt).toLocaleString(undefined, { month: "short", day: "numeric", year: "numeric", hour: "2-digit", minute: "2-digit" }); } catch { return p.createdAt; } })()}
                    </span>
                    <span className={`workflow-badge workflow-badge--${p.proposedStatus}`}>{p.proposedStatus.replace(/_/g, " ")}</span>
                    {p.proposedComment && <span className="workflow-history-comment">{p.proposedComment}</span>}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {isEmpty ? (
          <div className="empty-state">
            <div style={{ fontSize: 32 }}>🎫</div>
            <div className="empty-state-title">Nothing here yet</div>
            <div>Add tickets and Slack channels from the <strong>Edit</strong> form, then click <strong>Refresh</strong> to sync</div>
          </div>
        ) : (
          <div className="detail-sections">
            {/* Jira section */}
            {issues.length > 0 && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">
                    Jira
                    {breakdown && (breakdown.issues + breakdown.comments) > 0 && (
                      <span className="unread-badge" style={{ marginLeft: 6 }}>
                        {breakdown.issues + breakdown.comments} new
                      </span>
                    )}
                  </span>
                  <span className="detail-section-count">{issues.length}</span>
                </div>
                <div className="issue-list">
                  {issues.map((issue) => (
                    <IssueCard
                      key={issue.id}
                      issue={issue}
                      onDeleteIssue={handleDeleteIssue}
                      onDeleteComment={handleDeleteComment}
                      onToggleIssue={(id) => void handleToggleIssue(id)}
                      onToggleComment={(id) => void handleToggleComment(id)}
                      onMarkIssueViewed={async (iid) => { await MarkIssueViewed(iid); load(true); void loadBreakdown(); }}
                      onMarkCommentViewed={async (cid) => { await MarkCommentViewed(cid); load(true); void loadBreakdown(); }}
                      jiraBaseUrl={detail?.jira?.baseUrl}
                    />
                  ))}
                </div>
              </section>
            )}

            {/* Slack section */}
            {(slackChannels.length > 0 || slackThreads.length > 0 || slackDMConfigs.length > 0) && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">
                    Slack
                    {breakdown && (breakdown.threads + breakdown.dms) > 0 && (
                      <span className="unread-badge" style={{ marginLeft: 6 }}>
                        {breakdown.threads + breakdown.dms} new
                      </span>
                    )}
                  </span>
                  <span className="detail-section-count">
                    {slackChannels.length + slackThreads.length + slackDMConfigs.length}
                  </span>
                </div>

                <div className="issue-list">
                  {/* One card per configured channel, threads grouped under it */}
                  {slackChannels.map((ch) => (
                    <SlackChannelCard
                      key={ch.id}
                      channel={ch}
                      threads={slackThreads.filter((t) => t.channelId === ch.channelId)}
                      onDeleteChannel={handleDeleteChannel}
                      onDeleteThread={handleDeleteThread}
                      onToggleChannel={(id) => void handleToggleChannel(id)}
                      onToggleThread={(id) => void handleToggleThread(id)}
                      onMarkThreadViewed={async (tid) => { await MarkSlackThreadViewed(tid); load(true); void loadBreakdown(); }}
                    />
                  ))}

                  {/* Orphan threads (no matching configured channel) */}
                  {slackThreads
                    .filter((t) => !slackChannels.some((ch) => ch.channelId === t.channelId))
                    .map((t) => (
                      <SlackChannelCard
                        key={t.id}
                        channel={{ id: t.id, workspace: t.workspace, channelId: t.channelId, channelName: t.channelName, createdAt: t.createdAt, isActive: t.isActive }}
                        threads={[t]}
                        onDeleteThread={handleDeleteThread}
                        onToggleThread={(id) => void handleToggleThread(id)}
                        onMarkThreadViewed={async (tid) => { await MarkSlackThreadViewed(tid); load(true); void loadBreakdown(); }}
                      />
                    ))}

                  {/* DM configs */}
                  {slackDMConfigs.map((dm) => (
                    <SlackDMConfigCard
                      key={dm.id}
                      dm={dm}
                      onDelete={(id) => void handleDeleteDMConfig(id)}
                      onToggle={(id) => void handleToggleDMConfig(id)}
                      onDeleteEntry={(dmConfigId, entryId) => void handleDeleteDMEntry(dmConfigId, entryId)}
                      onMarkViewed={async (did) => { await MarkSlackDMConfigViewed(did); load(true); void loadBreakdown(); }}
                    />
                  ))}
                </div>
              </section>
            )}
            {/* Texts & Files section */}
            {(contextTexts.length > 0 || uploadedFiles.length > 0) && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">Texts & Files</span>
                  <span className="detail-section-count">{contextTexts.length + uploadedFiles.length}</span>
                </div>
                <div className="issue-list">
                  {contextTexts.map((f) => (
                    <ContextTextCard
                      key={f.id}
                      text={f}
                      onDelete={(id) => void handleDeleteContextText(id)}
                      onToggle={(id) => void handleToggleContextText(id)}
                    />
                  ))}
                  {uploadedFiles.map((f) => (
                    <UploadedFileCard
                      key={f.id}
                      file={f}
                      onDelete={(id) => void handleDeleteUploadedFile(id)}
                      onToggle={(id) => void handleToggleUploadedFile(id)}
                    />
                  ))}
                </div>
              </section>
            )}

            {/* Pull Requests section — grouped by repo/branch */}
            {(vcsBranches.length > 0 || pullRequests.length > 0) && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">
                    Pull Requests
                    {breakdown && (breakdown.prs + breakdown.prComments + breakdown.notifications) > 0 && (
                      <span className="unread-badge" style={{ marginLeft: 6 }}>
                        {breakdown.prs + breakdown.prComments + breakdown.notifications} new
                      </span>
                    )}
                  </span>
                  <span className="detail-section-count">{pullRequests.length}</span>
                </div>
                <div className="issue-list">
                  {vcsBranches.map((b) => {
                    const repoUrl = b.repo
                      ? `https://github.com/${b.repo.owner}/${b.repo.name}`
                      : null;
                    const branchUrl = repoUrl && b.branch
                      ? `${repoUrl}/tree/${b.branch}`
                      : repoUrl;
                    const repoLabel = b.repo
                      ? `${b.repo.owner}/${b.repo.name}`
                      : b.id;
                    const branchPRs = pullRequests.filter((pr) => pr.branchId === b.id);

                    return (
                      <div key={b.id} className="pr-repo-group">
                        {/* Repo / branch sub-header */}
                        <div className="pr-repo-group-header">
                          <span className="pr-repo-group-name">
                            {repoUrl ? (
                              <button
                                type="button"
                                className="pr-repo-link"
                                onClick={() => BrowserOpenURL(repoUrl)}
                                title={`Open ${repoLabel} on GitHub`}
                              >
                                {repoLabel}
                              </button>
                            ) : repoLabel}
                          </span>
                          {b.label && (
                            <span className="vcs-label-badge" title="Repo label">{b.label}</span>
                          )}
                          {b.branch && branchUrl && (
                            <button
                              type="button"
                              className="vcs-branch-chip vcs-branch-chip--link"
                              onClick={() => BrowserOpenURL(branchUrl)}
                              title={`Open branch ${b.branch} on GitHub`}
                            >
                              <GitBranch size={11} /> {b.branch}
                            </button>
                          )}
                          {branchPRs.length > 0 && (
                            <span className="detail-section-count" style={{ marginLeft: "auto" }}>
                              {branchPRs.length}
                            </span>
                          )}
                        </div>

                        {/* PRs for this branch */}
                        {branchPRs.length === 0 ? (
                          <div className="detail-section-empty" style={{ padding: "8px 14px 12px" }}>
                            Click <strong>Refresh</strong> to fetch PRs
                          </div>
                        ) : (
                          branchPRs.map((pr) => (
                            <PRCard
                              key={pr.id}
                              pr={pr}
                              vcsBranches={vcsBranches}
                              onMarkPRViewed={async (pid) => { await MarkPRViewed(pid); load(true); void loadBreakdown(); }}
                              onMarkCommentViewed={async (cid) => { await MarkPRCommentViewed(cid); load(true); void loadBreakdown(); }}
                            />
                          ))
                        )}
                      </div>
                    );
                  })}

                  {/* PRs that don't match any tracked branch (edge case) */}
                  {pullRequests
                    .filter((pr) => !vcsBranches.some((b) => b.id === pr.branchId))
                    .map((pr) => (
                      <PRCard
                        key={pr.id}
                        pr={pr}
                        vcsBranches={vcsBranches}
                        onMarkPRViewed={async (pid) => { await MarkPRViewed(pid); load(true); void loadBreakdown(); }}
                        onMarkCommentViewed={async (cid) => { await MarkPRCommentViewed(cid); load(true); void loadBreakdown(); }}
                      />
                    ))}
                </div>
              </section>
            )}
            {/* Relations section */}
            {(detail?.relations ?? []).length > 0 && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">Relations</span>
                  <span className="detail-section-count">{detail!.relations!.length}</span>
                </div>
                <div className="relations-list">
                  {(detail.relations ?? []).map((rel) => (
                    <div key={rel.id} className="relation-item">
                      <span className="relation-type-badge">
                        {RELATION_TYPE_LABELS[rel.relationType]}
                      </span>
                      <button
                        type="button"
                        className="btn-ghost relation-link"
                        onClick={() => navigate(`/context/${rel.targetId}`)}
                      >
                        {rel.targetKey}
                      </button>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
        </div>

        {id && (
          <aside className="detail-sidepanel">
            <div className="sidepanel-tabs">
              <button
                className={`sidepanel-tab${sidePanelTab === "chat" ? " sidepanel-tab--active" : ""}`}
                onClick={() => setSidePanelTab("chat")}
              >
                Chat
              </button>
              <button
                className={`sidepanel-tab${sidePanelTab === "logs" ? " sidepanel-tab--active" : ""}`}
                onClick={() => setSidePanelTab("logs")}
              >
                Logs
              </button>
              <button
                className={`sidepanel-tab${sidePanelTab === "consolidated" ? " sidepanel-tab--active" : ""}`}
                onClick={() => setSidePanelTab("consolidated")}
              >
                Consolidated
              </button>
            </div>
            <div className="sidepanel-body">
              {sidePanelTab === "chat" && (
                <ContextChatPanel
                  contextId={id}
                  onConsolidatedUpdated={() => setConsolidatedReloadKey((k) => k + 1)}
                />
              )}
              {sidePanelTab === "logs" && (
                <ContextLogList
                  contextId={id}
                  onChanged={() => setConsolidatedReloadKey((k) => k + 1)}
                />
              )}
              {sidePanelTab === "consolidated" && (
                <ConsolidatedLogView contextId={id} reloadKey={consolidatedReloadKey} />
              )}
            </div>
          </aside>
        )}
      </div>

      {editOpen && detail && (
        <ContextForm
          initial={detail.context as Context}
          initialJqlQuery={editJqlQuery}
          initialJiraIssues={editJiraIssues}
          initialSlackChannels={editSlackChannels}
          initialSlackDMConfigs={editSlackDMConfigs}
          initialContextTexts={editContextTexts}
          initialUploadedFiles={editUploadedFiles}
          initialVCSBranches={editVCSBranches}
          initialRelations={editRelations}
          onSave={handleUpdate}
          onCancel={closeEditForm}
        />
      )}
    </div>
  );
}
