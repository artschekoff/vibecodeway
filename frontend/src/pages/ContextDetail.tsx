import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import Spinner from "../components/Spinner";
import { useTabContext } from "../contexts/TabContext";
import { ArrowLeft, Check, CheckCheck, ChevronDown, Copy, ExternalLink, GitBranch, GitPullRequest, Pencil, RefreshCw, Trash2, UserCheck, X } from "lucide-react";
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
  ListAllWorkspaces,
  DeleteWorkspace,
  MarkWorkspaceShouldDelete,
  // @ts-ignore
  AddContextRelation,
  // @ts-ignore
  RemoveContextRelation,
  GetUnviewedBreakdown,
  MarkContextAllViewed,
  MarkIssueViewed,
  MarkIssueAllViewed,
  MarkCommentViewed,
  MarkSlackThreadViewed,
  MarkSlackChannelAllViewed,
  MarkSlackDMConfigViewed,
  MarkPRViewed,
  MarkPRCommentViewed,
  MarkBranchAllViewed,
  AnalyzeContext,
  GetContextSnapshot,
  GetEstimationHistory,
  GenerateHandoffPacket,
  AddUserEvent,
  DeleteUserEvent,
  UpdateUserEvent,
  ListEvents,
  GetJiraInstanceURL,
  GetPersonAssignments,
  GeneratePersonAssignments,
  GenerateSinglePersonAssignment,
  GetContextParticipants,
  RefinePersonAssignment,
  SavePersonAssignment,
  DeletePersonAssignment,
  GetPersonAssignmentSteps,
  SetCurrentAssignmentStep,
  RebuildAssignmentChainFrom,
  RegeneratePersonAssignment,
  GetPendingMessages,
  MarkAssignmentsOutdated,
  GenerateEventsFromMessages,
  SetContextWorkflowStatus,
  ListWorkflowStatuses,
  GetUnresolvedAuthors,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL, EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { useCheckout } from "../lib/useCheckout";
import type { service } from "../../wailsjs/go/models";
import type { Context, Issue, SlackChannel, ContextText, SlackDMConfig, SlackSavePayload, TextFilesPayload, UploadedFile, VCSBranch, PullRequest, UnviewedBreakdown, ContextRelation, RelationsPayload, ContextSnapshot, ContextEvent, VCSWorkspace, EstimationLog, PersonAssignment, PersonAssignmentStep, ContextMessage, WorkflowStatus } from "../types";
import { RELATION_TYPE_LABELS } from "../types";
import type { VCSBranchInput } from "../components/ContextForm";
import IssueCard from "../components/IssueCard";
import SlackChannelCard from "../components/SlackChannelCard";
import SlackDMConfigCard from "../components/SlackDMConfigCard";
import UploadedFileCard from "../components/UploadedFileCard";
import ContextTextCard from "../components/ContextTextCard";
import ContextForm from "../components/ContextForm";
import EstimationPanel from "../components/EstimationPanel";
import { UnresolvedAuthorsModal } from "../components/UnresolvedAuthorsModal";
import { InstructionInput } from "../components/InstructionInput";
import { ReasoningChainGraph } from "../components/ReasoningChainGraph";
import { useProgress } from "../contexts/ProgressContext";
import { usePassphrase } from "../contexts/PassphraseContext";
import WorkflowStatusBadge from "../components/WorkflowStatusBadge";

function PRStatusBadge({ status, reviewStatus, mergeable, ciStatus, isDraft }: { status: string; reviewStatus: string; mergeable?: string; ciStatus?: string; isDraft?: boolean }) {
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
      {isDraft && <span className="pr-status-badge pr-status-badge--draft">Draft</span>}
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

function PRCard({ pr, vcsBranches, onMarkPRViewed, onMarkCommentViewed, checkoutNode }: {
  pr: PullRequest;
  vcsBranches?: VCSBranch[];
  onMarkPRViewed?: (id: string) => void;
  onMarkCommentViewed?: (id: string) => void;
  checkoutNode?: React.ReactNode;
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
          <GitPullRequest size={13} className="pr-type-icon" />
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
          {checkoutNode && (
            <span onClick={(e) => e.stopPropagation()}>{checkoutNode}</span>
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
        <PRStatusBadge status={pr.state} reviewStatus={pr.reviewStatus} mergeable={pr.mergeable} ciStatus={pr.ciStatus} isDraft={pr.isDraft} />
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

interface ContextDetailProps {
  contextId: string;
  tabId: string;
}

function WorkspaceMenu({
  workspace,
  checkoutLoading,
  onMarkDone,
  onDelete,
  onCheckout,
}: {
  workspace: import("../types").VCSWorkspace;
  checkoutLoading: boolean;
  onMarkDone: (id: string) => void;
  onDelete: (id: string) => void;
  onCheckout: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
        setConfirmDelete(false);
      }
    };
    if (open) document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  return (
    <div className="workspace-menu" ref={ref}>
      <button
        type="button"
        className="workspace-menu-trigger"
        onClick={() => { setOpen((v) => !v); setConfirmDelete(false); }}
        disabled={checkoutLoading}
        title="Workspace actions"
      >
        <span className={`workspace-status-badge workspace-status-badge--${workspace.status}`}>
          {workspace.status === "should_delete" ? "done" : "active"}
        </span>
        {checkoutLoading ? <Spinner size={10} /> : <ChevronDown size={10} />}
      </button>
      {open && (
        <div className="workspace-menu-dropdown">
          <button
            className="workspace-menu-item"
            onClick={() => { onCheckout(); setOpen(false); }}
          >
            Open workspace
          </button>
          {workspace.status === "active" && (
            <button
              className="workspace-menu-item"
              onClick={() => { onMarkDone(workspace.id); setConfirmDelete(false); }}
            >
              Done
            </button>
          )}
          <div className="workspace-menu-divider" />
          {confirmDelete ? (
            <div className="workspace-menu-confirm">
              <span>Really delete?</span>
              <button
                className="workspace-menu-item workspace-menu-item--danger"
                onClick={() => { onDelete(workspace.id); setOpen(false); setConfirmDelete(false); }}
              >
                Yes, delete
              </button>
              <button
                className="workspace-menu-item"
                onClick={() => setConfirmDelete(false)}
              >
                Cancel
              </button>
            </div>
          ) : (
            <button
              className="workspace-menu-item workspace-menu-item--danger"
              onClick={() => setConfirmDelete(true)}
            >
              Delete workspace
            </button>
          )}
        </div>
      )}
    </div>
  );
}

const AVATAR_COLORS = ['#6366f1', '#8b5cf6', '#ec4899', '#f43f5e', '#f97316', '#eab308', '#22c55e', '#14b8a6', '#06b6d4', '#3b82f6'];
function getAvatarColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) & 0xfffffff;
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

export default function ContextDetailPage({ contextId, tabId: _tabId }: ContextDetailProps) {
  const id = contextId;
  const { closeTab, navigateBack, openContextTab } = useTabContext();
  const progress = useProgress();
  const { ensurePassphrase } = usePassphrase();
  const [detail, setDetail] = useState<service.ContextDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshError, setRefreshError] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [toastMsg, setToastMsg] = useState("");
  const [toastType, setToastType] = useState<"success" | "error">("success");
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
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
  const [snapshot, setSnapshot] = useState<ContextSnapshot | null>(null);
  const [workflowStatuses, setWorkflowStatuses] = useState<WorkflowStatus[]>([]);
  const [showStatusChanger, setShowStatusChanger] = useState(false);
  const [statusChangeSlug, setStatusChangeSlug] = useState("");
  const [statusChangeComment, setStatusChangeComment] = useState("");
  const [isChangingStatus, setIsChangingStatus] = useState(false);
  const [estimationHistory, setEstimationHistory] = useState<EstimationLog[]>([]);
  const [events, setEvents] = useState<ContextEvent[]>([]);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [snapshotError, setSnapshotError] = useState("");
  const [addEventType, setAddEventType] = useState("user_note");
  const [addEventSummary, setAddEventSummary] = useState("");
  const [personAssignments, setPersonAssignments] = useState<PersonAssignment[]>([]);
  const [isGeneratingAssignments, setIsGeneratingAssignments] = useState(false);
  const [editingPersonId, setEditingPersonId] = useState<string | null>(null);
  const [editPersonBody, setEditPersonBody] = useState("");
  const [isSavingPerson, setIsSavingPerson] = useState(false);
  const [refiningPersonId, setRefiningPersonId] = useState<string | null>(null);
  const [personAiPrompt, setPersonAiPrompt] = useState("");
  const [isRefiningPerson, setIsRefiningPerson] = useState(false);
  const [stepsMap, setStepsMap] = useState<Record<string, PersonAssignmentStep[]>>({});
  const [reasoningGraphFor, setReasoningGraphFor] = useState<string | null>(null);
  const [expandedChainFor, setExpandedChainFor] = useState<string | null>(null);
  const [pendingMap, setPendingMap] = useState<Record<string, ContextMessage[]>>({});
  const [loadingStepsFor, setLoadingStepsFor] = useState<string | null>(null);
  const [regeneratingFor, setRegeneratingFor] = useState<string | null>(null);
  const [rebuildingFrom, setRebuildingFrom] = useState<{ paId: string; stepIndex: number } | null>(null);
  const [editStepContent, setEditStepContent] = useState<Record<string, string>>({});
  const [editingStepKey, setEditingStepKey] = useState<string | null>(null);
  const [promotingStep, setPromotingStep] = useState<string | null>(null);
  const [participants, setParticipants] = useState<string[]>([]);
  const [isLoadingParticipants, setIsLoadingParticipants] = useState(false);
  const [expandedPersonName, setExpandedPersonName] = useState<string | null>(null);
  const [participantCustomPrompts, setParticipantCustomPrompts] = useState<Record<string, string>>({});
  const [generatingForPerson, setGeneratingForPerson] = useState<string | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);
  const [showAuthorModal, setShowAuthorModal] = useState(false);
  const [editingEventId, setEditingEventId] = useState<string | null>(null);
  const [editEventType, setEditEventType] = useState("user_note");
  const [editEventSummary, setEditEventSummary] = useState("");
  const [jiraInstanceUrl, setJiraInstanceUrl] = useState<string>("");
  const [workspaces, setWorkspaces] = useState<VCSWorkspace[]>([]);
  const {
    checkout: handleCheckout,
    checkoutWithPath: handleCheckoutWithPath,
    dismissCheckoutModal,
    checkoutTarget,
    checkoutPaths,
    checkoutLoading,
    checkoutError,
    setCheckoutError,
  } = useCheckout(
    (ws) => setWorkspaces(ws),
    "No local path set for this repository. Add one in the Repositories tab."
  );

  const allPersons = useMemo(() => {
    const names = new Set<string>([...participants, ...personAssignments.map(pa => pa.name)]);
    return Array.from(names).map(name => ({ name, assignment: personAssignments.find(pa => pa.name === name) ?? null }));
  }, [participants, personAssignments]);

  const loadSteps = async (paId: string) => {
    setLoadingStepsFor(paId);
    try {
      const steps = await GetPersonAssignmentSteps(paId) as PersonAssignmentStep[] | null;
      setStepsMap(p => ({ ...p, [paId]: steps ?? [] }));
    } finally { setLoadingStepsFor(null); }
  };

  const loadPending = async (paId: string) => {
    try {
      const msgs = await GetPendingMessages(paId) as ContextMessage[] | null;
      setPendingMap(p => ({ ...p, [paId]: msgs ?? [] }));
    } catch { /* non-critical */ }
  };

  const loadBreakdown = useCallback(async () => {
    if (!id) return;
    try {
      const bd = await GetUnviewedBreakdown(id);
      setBreakdown(bd);
    } catch {
      // best-effort
    }
  }, [id]);

  const loadSnapshot = useCallback(async () => {
    if (!id) return;
    try {
      const [snap, evts, pas] = await Promise.all([
        GetContextSnapshot(id) as Promise<ContextSnapshot>,
        ListEvents(id) as Promise<ContextEvent[]>,
        GetPersonAssignments(id).catch(() => []) as Promise<PersonAssignment[]>,
      ]);
      setSnapshot(snap);
      setEvents(evts ?? []);
      setPersonAssignments(pas ?? []);
    } catch {
      // snapshot not yet generated — that's fine
    }
    GetEstimationHistory(id)
      .then((h) => setEstimationHistory(h ?? []))
      .catch(() => setEstimationHistory([]));
  }, [id]);

  const analyzeContext = async () => {
    setIsAnalyzing(true);
    setSnapshotError("");
    try {
      const result = await AnalyzeContext(id) as ContextSnapshot;
      setSnapshot(result);
      const evts = await ListEvents(id) as ContextEvent[];
      setEvents(evts ?? []);
      GetEstimationHistory(id)
        .then((h) => setEstimationHistory(h ?? []))
        .catch(() => {});
    } catch (e) {
      setSnapshotError(String(e));
    } finally {
      setIsAnalyzing(false);
    }
  };

  const handleStatusChange = async () => {
    if (!statusChangeSlug) return;
    setIsChangingStatus(true);
    try {
      const updated = await SetContextWorkflowStatus(id, statusChangeSlug, statusChangeComment);
      if (updated) setSnapshot(updated as ContextSnapshot);
      setShowStatusChanger(false);
      setStatusChangeSlug("");
      setStatusChangeComment("");
    } finally {
      setIsChangingStatus(false);
    }
  };

  const load = useCallback(async (silent = false) => {
    if (!id) return;
    if (!silent) setLoading(true);
    try {
      const d = await GetContextDetail(id);
      setDetail(d);
      void loadBreakdown();
      const ws = (await ListAllWorkspaces() as VCSWorkspace[]) ?? [];
      setWorkspaces(ws);
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
    void loadSnapshot();
  }, [load, loadSnapshot]);

  useEffect(() => {
    GetJiraInstanceURL().then(setJiraInstanceUrl).catch(() => {});
  }, []);

  useEffect(() => {
    void ListWorkflowStatuses().then((ss) => setWorkflowStatuses(ss ?? []));
  }, [id]);

  const handleRefresh = async () => {
    if (!id) return;
    setRefreshing(true);
    setRefreshError("");
    setToastMsg("");

    const hasSlack =
      (detail?.slackChannels?.length ?? 0) > 0 ||
      (detail?.slackDMConfigs?.length ?? 0) > 0;
    if (hasSlack && !(await ensurePassphrase())) {
      setRefreshing(false);
      return;
    }

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
      void loadSnapshot();
      GetUnresolvedAuthors(id).then((authors) => {
        if (authors && authors.length > 0) setShowAuthorModal(true);
      }).catch(() => {});

      try {
        const [pendingCounts] = await Promise.all([
          MarkAssignmentsOutdated(id) as Promise<Record<string, number> | null>,
          GenerateEventsFromMessages(id),
        ]);
        if (pendingCounts) {
          const updatedPAs = await GetPersonAssignments(id) as PersonAssignment[] | null;
          if (updatedPAs) setPersonAssignments(updatedPAs);
          for (const [paId, count] of Object.entries(pendingCounts)) {
            if (count > 0) void loadPending(paId);
          }
        }
      } catch { /* non-critical */ }
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
        try { await AddSlackChannels(id, group.workspace, group.channels as any); } catch { /* best-effort */ }
      }
      for (const dmId of removedDMConfigIds) {
        try { await DeleteSlackDMConfig(dmId); } catch { /* best-effort */ }
      }
      if (dmConfigs.length > 0 && !(await ensurePassphrase())) {
        // skip DM upserts — user cancelled passphrase prompt
      } else {
        for (const dm of dmConfigs) {
          progress.next(`Loading DMs from ${dm.workspace}`);
          try { await UpsertSlackDMConfig(id, dm.workspace, dm.startFrom); } catch { /* best-effort */ }
        }
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

  const handleDeleteWorkspace = async (workspaceId: string) => {
    try {
      await DeleteWorkspace(workspaceId);
      setWorkspaces((await ListAllWorkspaces() as VCSWorkspace[]) ?? []);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  const handleMarkWorkspaceDone = async (workspaceId: string) => {
    try {
      await MarkWorkspaceShouldDelete(workspaceId);
      setWorkspaces((await ListAllWorkspaces() as VCSWorkspace[]) ?? []);
    } catch (e) {
      showToast(String(e), "error");
    }
  };

  if (loading) return <div className="loading">Loading…</div>;

  if (!detail) return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => navigateBack(_tabId)}><ArrowLeft size={16} /> Back</button>
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
        <button className="back-btn" onClick={() => navigateBack(_tabId)}><ArrowLeft size={16} /> Back</button>
        <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
          <h1 style={{ fontSize: 16, margin: 0, whiteSpace: "nowrap", display: "flex", alignItems: "center", gap: 6 }}>
            {context.key}
            <button
              style={{ background: "none", border: "none", padding: 0, cursor: "pointer", color: "var(--text-muted)", display: "flex", alignItems: "center", lineHeight: 1 }}
              title="Copy context name"
              onClick={() => { void navigator.clipboard.writeText(context.key); showToast("Copied!"); }}
            >
              <Copy size={13} />
            </button>
          </h1>
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
            onClick={() => setShowAuthorModal(true)}
            title="Assign unknown authors"
          >
            <UserCheck size={16} />
          </button>
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
        {/* Analysis snapshot */}
        <div className="detail-section" style={{ marginBottom: 12 }}>
          <div className="detail-section-header">
            <span className="detail-section-title">Analysis</span>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              {snapshot?.isStale && (
                <span style={{ color: "orange", fontSize: 12 }}>New data — re-analyze</span>
              )}
              <button className="btn-secondary" style={{ fontSize: 12, padding: "2px 10px" }} onClick={() => void analyzeContext()} disabled={isAnalyzing}>
                {isAnalyzing ? "Analyzing…" : "Analyze"}
              </button>
            </div>
          </div>
          {snapshotError && <div className="error-banner" style={{ margin: "0 14px 8px" }}>{snapshotError}</div>}
          <div style={{ padding: "8px 14px" }}>
            {snapshot ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                <div>
                  <strong>Current step:</strong> {snapshot.currentStepDesc}{" "}
                  <em style={{ color: "var(--text-muted)", fontSize: 12 }}>({snapshot.currentStepConfidence})</em>
                </div>
                <div>
                  <strong>Status:</strong>{" "}
                  <div style={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 6 }}>
                    <WorkflowStatusBadge status={snapshot.status} statuses={workflowStatuses} />
                    {snapshot.latestStatusLogEntry?.comment && (
                      <span style={{ fontSize: 12, color: "var(--text-muted)" }}>
                        — {snapshot.latestStatusLogEntry.comment}
                      </span>
                    )}
                    <button
                      className="btn-ghost"
                      style={{ fontSize: 11, padding: "1px 7px" }}
                      onClick={() => {
                        setStatusChangeSlug(snapshot.status);
                        setStatusChangeComment("");
                        setShowStatusChanger((v) => !v);
                      }}
                    >
                      Change
                    </button>
                  </div>
                  {showStatusChanger && (
                    <div style={{ marginTop: 8, display: "flex", flexDirection: "column", gap: 6 }}>
                      <select
                        className="form-control"
                        style={{ fontSize: 12 }}
                        value={statusChangeSlug}
                        onChange={(e) => setStatusChangeSlug(e.target.value)}
                      >
                        {workflowStatuses.map((st) => (
                          <option key={st.slug} value={st.slug}>{st.label}</option>
                        ))}
                      </select>
                      <input
                        className="form-control"
                        style={{ fontSize: 12 }}
                        placeholder="Reason for this status change…"
                        value={statusChangeComment}
                        onChange={(e) => setStatusChangeComment(e.target.value)}
                      />
                      <div style={{ display: "flex", gap: 6 }}>
                        <button
                          className="btn-primary"
                          style={{ fontSize: 12 }}
                          disabled={!statusChangeSlug || isChangingStatus}
                          onClick={() => void handleStatusChange()}
                        >
                          {isChangingStatus ? "Saving…" : "Save"}
                        </button>
                        <button
                          className="btn-ghost"
                          style={{ fontSize: 12 }}
                          onClick={() => setShowStatusChanger(false)}
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  )}
                </div>
                {snapshot.sectionOperational && (
                  <details open>
                    <summary style={{ cursor: "pointer", fontWeight: 500, fontSize: 13 }}>Operational</summary>
                    <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: "6px 0 0", color: "var(--text-muted)" }}>{snapshot.sectionOperational}</pre>
                  </details>
                )}
                {snapshot.sectionNarrative && (
                  <details>
                    <summary style={{ cursor: "pointer", fontWeight: 500, fontSize: 13 }}>Narrative</summary>
                    <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: "6px 0 0", color: "var(--text-muted)" }}>{snapshot.sectionNarrative}</pre>
                  </details>
                )}
                {snapshot.sectionEngineering && (
                  <details>
                    <summary style={{ cursor: "pointer", fontWeight: 500, fontSize: 13 }}>Engineering</summary>
                    <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: "6px 0 0", color: "var(--text-muted)" }}>{snapshot.sectionEngineering}</pre>
                  </details>
                )}
                {snapshot && (
                  <div className="context-detail__estimation">
                    <h3 className="context-detail__section-title">Time Estimate</h3>
                    <EstimationPanel snapshot={snapshot} history={estimationHistory} />
                  </div>
                )}
                {(snapshot.aiNextStepsShort || snapshot.aiNextStepsMid || snapshot.aiNextStepsLong) && (
                  <div style={{ marginTop: 4 }}>
                    <strong style={{ fontSize: 13 }}>Next steps (AI):</strong>
                    {snapshot.aiNextStepsShort && <div style={{ fontSize: 12 }}><em>Today:</em> {snapshot.aiNextStepsShort}</div>}
                    {snapshot.aiNextStepsMid && <div style={{ fontSize: 12 }}><em>This week:</em> {snapshot.aiNextStepsMid}</div>}
                    {snapshot.aiNextStepsLong && <div style={{ fontSize: 12 }}><em>This month:</em> {snapshot.aiNextStepsLong}</div>}
                  </div>
                )}
                <div style={{ display: "flex", gap: 8, marginTop: 4 }}>
                  <button
                    className="btn-secondary"
                    style={{ fontSize: 12, padding: "2px 10px" }}
                    onClick={async () => {
                      try {
                        const packet = await GenerateHandoffPacket(id);
                        await navigator.clipboard.writeText(JSON.stringify(packet, null, 2));
                      } catch { /* best-effort */ }
                    }}
                  >
                    Copy Handoff Packet
                  </button>
                </div>
                {/* Person Assignments */}
                <details style={{ marginTop: 8 }}>
                  <summary style={{ cursor: "pointer", fontWeight: 500, fontSize: 13, userSelect: "none" }}>
                    Person Assignments
                    {personAssignments.length > 0 && (
                      <span style={{ fontWeight: 400, color: "var(--text-muted)", marginLeft: 6, fontSize: 11 }}>
                        ({personAssignments.length})
                      </span>
                    )}
                  </summary>
                  <div style={{ marginTop: 8, display: "flex", flexDirection: "column", gap: 8 }}>

                    {/* Avatar row */}
                    <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: 6 }}>
                      {allPersons.map((person) => {
                        const isActive = expandedPersonName === person.name;
                        const isGenerating = generatingForPerson === person.name;
                        return (
                          <button
                            key={person.name}
                            onClick={() => setExpandedPersonName(isActive ? null : person.name)}
                            style={{
                              background: isActive ? "var(--bg-secondary)" : "none",
                              border: isActive ? "1px solid var(--accent)" : "1px solid transparent",
                              borderRadius: 8, cursor: "pointer", padding: "5px 7px",
                              display: "flex", flexDirection: "column", alignItems: "center", gap: 3,
                            }}
                            title={person.name}
                          >
                            <div style={{ position: "relative" }}>
                              <div style={{
                                width: 36, height: 36, borderRadius: "50%",
                                background: getAvatarColor(person.name),
                                display: "flex", alignItems: "center", justifyContent: "center",
                                color: "white", fontWeight: 700, fontSize: 15,
                                opacity: isGenerating ? 0.5 : 1,
                              }}>
                                {isGenerating ? <Spinner size={16} /> : person.name[0].toUpperCase()}
                              </div>
                              <div style={{
                                position: "absolute", bottom: -1, right: -1,
                                width: 13, height: 13, borderRadius: "50%",
                                background: person.assignment ? "#22c55e" : "var(--surface2)",
                                border: "2px solid var(--bg)",
                                display: "flex", alignItems: "center", justifyContent: "center",
                              }}>
                                {person.assignment && <Check size={7} color="white" strokeWidth={3} />}
                              </div>
                            </div>
                            <span style={{ fontSize: 10, maxWidth: 60, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--text)" }}>
                              {person.name.split(" ")[0]}
                            </span>
                          </button>
                        );
                      })}
                      <button
                        className="btn-secondary"
                        style={{ fontSize: 10, padding: "2px 8px", alignSelf: "flex-start", marginTop: 2, opacity: 0.7 }}
                        disabled={isLoadingParticipants}
                        onClick={async () => {
                          setIsLoadingParticipants(true);
                          try {
                            const names = await GetContextParticipants(id) as string[];
                            setParticipants(names ?? []);
                          } finally {
                            setIsLoadingParticipants(false);
                          }
                        }}
                      >
                        {isLoadingParticipants ? "Loading…" : allPersons.length > 0 ? "↻ Refresh" : "+ Load people"}
                      </button>
                    </div>

                    {/* Expanded person panel */}
                    {expandedPersonName && (() => {
                      const personData = allPersons.find(p => p.name === expandedPersonName);
                      const pa = personData?.assignment ?? null;
                      const customPrompt = participantCustomPrompts[expandedPersonName] ?? "";
                      return (
                        <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
                          {/* Panel header */}
                          <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 10px", background: "var(--bg-secondary)", borderBottom: "1px solid var(--border)" }}>
                            <div style={{
                              width: 24, height: 24, borderRadius: "50%", flexShrink: 0,
                              background: getAvatarColor(expandedPersonName),
                              display: "flex", alignItems: "center", justifyContent: "center",
                              color: "white", fontWeight: 700, fontSize: 11,
                            }}>
                              {expandedPersonName[0].toUpperCase()}
                            </div>
                            <span style={{ fontWeight: 600, fontSize: 13, flex: 1 }}>{expandedPersonName}</span>
                            {pa && editingPersonId !== pa.id && refiningPersonId !== pa.id && (
                              <>
                                <button
                                  style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1, fontSize: 11 }}
                                  title="Refine with AI"
                                  onClick={() => { setRefiningPersonId(pa.id); setPersonAiPrompt(""); }}
                                >
                                  AI
                                </button>
                                <button
                                  style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1, fontSize: 11 }}
                                  title="Re-generate"
                                  disabled={generatingForPerson === pa.name || regeneratingFor === pa.id}
                                  onClick={async () => {
                                    setGeneratingForPerson(pa.name);
                                    try {
                                      const updated = await GenerateSinglePersonAssignment(id, pa.name, "") as PersonAssignment;
                                      setPersonAssignments((prev) => prev.map((p) => p.id === pa.id ? updated : p));
                                    } finally { setGeneratingForPerson(null); }
                                  }}
                                >
                                  {generatingForPerson === pa.name || regeneratingFor === pa.id ? "…" : "↻"}
                                </button>
                                <button
                                  style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1 }}
                                  title="Delete"
                                  onClick={async () => {
                                    await DeletePersonAssignment(pa.id);
                                    setPersonAssignments((prev) => prev.filter((p) => p.id !== pa.id));
                                    setExpandedPersonName(null);
                                  }}
                                >
                                  <Trash2 size={12} />
                                </button>
                                <button
                                  style={{ background: "none", border: "1px solid var(--border)", borderRadius: 4, cursor: "pointer", fontSize: 11, color: "var(--text-muted)", padding: "2px 8px" }}
                                  onClick={async () => {
                                    setReasoningGraphFor(pa.id);
                                    if (!stepsMap[pa.id]) await loadSteps(pa.id);
                                  }}
                                >
                                  Show Reasoning
                                </button>
                              </>
                            )}
                            <button
                              style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1, marginLeft: 4 }}
                              onClick={() => setExpandedPersonName(null)}
                            >
                              <X size={14} />
                            </button>
                          </div>

                          <div style={{ padding: "8px 10px", display: "flex", flexDirection: "column", gap: 6 }}>
                            {/* Generate form when no assignment exists */}
                            {!pa && (
                              <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                                <div style={{ fontSize: 11, color: "var(--text-muted)" }}>No assignment yet.</div>
                                <textarea
                                  className="passphrase-input"
                                  style={{ fontSize: 12, minHeight: 52, resize: "vertical", width: "100%", fontFamily: "inherit" }}
                                  placeholder="Additional context (optional): e.g. Focus on backend API"
                                  value={customPrompt}
                                  onChange={(e) => setParticipantCustomPrompts((prev) => ({ ...prev, [expandedPersonName]: e.target.value }))}
                                />
                                <button
                                  className="btn-primary"
                                  style={{ fontSize: 12, padding: "2px 10px", alignSelf: "flex-start" }}
                                  disabled={generatingForPerson === expandedPersonName}
                                  onClick={async () => {
                                    const name = expandedPersonName;
                                    setGeneratingForPerson(name);
                                    try {
                                      const generated = await GenerateSinglePersonAssignment(id, name, customPrompt.trim()) as PersonAssignment;
                                      setPersonAssignments((prev) => {
                                        const idx = prev.findIndex((p) => p.id === generated.id || p.name === generated.name);
                                        return idx >= 0 ? prev.map((p, i) => i === idx ? generated : p) : [...prev, generated];
                                      });
                                    } finally { setGeneratingForPerson(null); }
                                  }}
                                >
                                  {generatingForPerson === expandedPersonName ? "Generating…" : `Generate for ${expandedPersonName}`}
                                </button>
                              </div>
                            )}

                            {/* Assignment content */}
                            {pa && (
                              <>
                                {/* Outdated banner */}
                                {pa.isOutdated && !pendingMap[pa.id] && (() => { void loadPending(pa.id); return null; })()}
                                {pa.isOutdated && (
                                  <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "4px 8px", background: "rgba(255,180,0,0.1)", border: "1px solid rgba(255,180,0,0.3)", borderRadius: 4, fontSize: 11 }}>
                                    <span style={{ color: "#f0a500" }}>⚠ Outdated</span>
                                    {(pendingMap[pa.id] ?? []).length > 0 && (
                                      <span style={{ color: "var(--text-muted)" }}>
                                        {(pendingMap[pa.id] ?? []).length} new message{(pendingMap[pa.id] ?? []).length !== 1 ? "s" : ""} not yet incorporated
                                      </span>
                                    )}
                                    <button
                                      className="btn-primary"
                                      style={{ fontSize: 10, padding: "2px 8px", marginLeft: "auto" }}
                                      disabled={regeneratingFor === pa.id || generatingForPerson === pa.name}
                                      onClick={async () => {
                                        setRegeneratingFor(pa.id);
                                        try {
                                          const updated = await RegeneratePersonAssignment(pa.id) as PersonAssignment;
                                          setPersonAssignments(prev => prev.map(p => p.id === pa.id ? updated : p));
                                          setPendingMap(p => ({ ...p, [pa.id]: [] }));
                                          void loadSteps(pa.id);
                                        } finally { setRegeneratingFor(null); }
                                      }}
                                    >
                                      {regeneratingFor === pa.id
                                        ? <span style={{ display: "flex", alignItems: "center", gap: 4 }}><span style={{ width: 8, height: 8, borderRadius: "50%", border: "2px solid rgba(255,255,255,0.3)", borderTopColor: "#fff", display: "inline-block", animation: "spin 0.6s linear infinite" }} />Re-generating…</span>
                                        : "Re-generate"}
                                    </button>
                                  </div>
                                )}

                                {editingPersonId === pa.id ? (
                                  <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                    <textarea
                                      style={{ fontSize: 12, minHeight: 180, resize: "vertical", width: "100%", fontFamily: "inherit", color: "var(--text-muted)", lineHeight: 1.5, margin: 0, textAlign: "left", letterSpacing: "normal" }}
                                      value={editPersonBody}
                                      onChange={(e) => setEditPersonBody(e.target.value)}
                                      autoFocus
                                    />
                                    <div style={{ display: "flex", gap: 4 }}>
                                      <button
                                        className="btn-primary"
                                        style={{ fontSize: 12, padding: "2px 10px", minWidth: 52 }}
                                        disabled={isSavingPerson}
                                        onClick={async () => {
                                          setIsSavingPerson(true);
                                          try {
                                            await SavePersonAssignment(pa.id, editPersonBody);
                                            setPersonAssignments((prev) => prev.map((p) => p.id === pa.id ? { ...p, body: editPersonBody } : p));
                                            setEditingPersonId(null);
                                          } finally {
                                            setIsSavingPerson(false);
                                          }
                                        }}
                                      >
                                        {isSavingPerson ? "Saving…" : "Save"}
                                      </button>
                                      <button className="btn-secondary" style={{ fontSize: 12, padding: "2px 10px" }} disabled={isSavingPerson} onClick={() => setEditingPersonId(null)}>Cancel</button>
                                    </div>
                                  </div>
                                ) : refiningPersonId === pa.id ? (
                                  <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                                    <InstructionInput
                                      contextId={id}
                                      value={personAiPrompt}
                                      onChange={setPersonAiPrompt}
                                      label={`Describe how to change ${pa.name}'s assignment`}
                                      isLoading={isRefiningPerson}
                                      onApply={async (fullPrompt) => {
                                        setIsRefiningPerson(true);
                                        try {
                                          const updated = await RefinePersonAssignment(id, pa.id, fullPrompt) as PersonAssignment;
                                          setPersonAssignments(prev => prev.map(p => p.id === pa.id ? updated : p));
                                          void loadSteps(pa.id);
                                          setRefiningPersonId(null);
                                          setPersonAiPrompt("");
                                        } finally { setIsRefiningPerson(false); }
                                      }}
                                      onCancel={() => { setRefiningPersonId(null); setPersonAiPrompt(""); }}
                                    />
                                    <pre style={{ whiteSpace: "pre-wrap", fontSize: 11, margin: "4px 0 0", color: "var(--text-muted)", lineHeight: 1.4, maxHeight: 200, overflow: "auto" }}>
                                      {pa.body}
                                    </pre>
                                  </div>
                                ) : (
                                  <pre
                                    style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: 0, color: "var(--text-muted)", lineHeight: 1.5, cursor: "text" }}
                                    title="Click to edit"
                                    onClick={() => { setEditingPersonId(pa.id); setEditPersonBody(pa.body); }}
                                  >
                                    {pa.body}
                                  </pre>
                                )}

                                {/* Reasoning chain */}
                                <div style={{ marginTop: 8, borderTop: "1px solid var(--border)", paddingTop: 6 }}>
                                  <button
                                    style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", fontSize: 10, padding: 0 }}
                                    onClick={async () => {
                                      if (expandedChainFor === pa.id) { setExpandedChainFor(null); return; }
                                      setExpandedChainFor(pa.id);
                                      if (!stepsMap[pa.id]) await loadSteps(pa.id);
                                    }}
                                  >
                                    {expandedChainFor === pa.id ? "▲ Hide chain" : `▶ Reasoning chain${stepsMap[pa.id] ? ` (${stepsMap[pa.id].length} steps)` : ""}`}
                                  </button>
                                  {expandedChainFor === pa.id && (
                                    <div style={{ marginTop: 6, display: "flex", flexDirection: "column", gap: 4 }}>
                                      {loadingStepsFor === pa.id && <div style={{ fontSize: 11, color: "var(--text-muted)" }}>Loading…</div>}
                                      {(stepsMap[pa.id] ?? []).map((step) => {
                                        const key = `${pa.id}-${step.stepIndex}`;
                                        const isRebuildingThis = rebuildingFrom?.paId === pa.id && rebuildingFrom.stepIndex === step.stepIndex;
                                        const sourceLabel = step.sourceType === "initial" ? "Initial generation" : step.sourceType === "instruction" ? "Instruction" : "New message";
                                        return (
                                          <div
                                            key={step.id}
                                            style={{
                                              borderLeft: step.isCurrent ? "3px solid var(--accent)" : "2px solid var(--border)",
                                              background: step.isCurrent ? "rgba(var(--accent-rgb, 99,102,241),0.04)" : "transparent",
                                              borderRadius: "0 4px 4px 0",
                                              padding: "4px 8px",
                                            }}
                                          >
                                            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                                              <span style={{ background: "var(--surface2)", borderRadius: 3, padding: "1px 5px", fontSize: 10, color: "var(--text-muted)" }}>{sourceLabel}</span>
                                              {step.isCurrent && <span style={{ fontSize: 10, color: "var(--accent)", fontWeight: 600 }}>CURRENT</span>}
                                              {step.qualityScore != null && (
                                                <span title={step.qualityNotes ?? ""} style={{
                                                  fontSize: 9, fontWeight: 600, padding: "1px 5px", borderRadius: 3,
                                                  background: step.qualityScore >= 0.7 ? "rgba(34,197,94,0.15)" : step.qualityScore >= 0.4 ? "rgba(234,179,8,0.15)" : "rgba(239,68,68,0.15)",
                                                  color: step.qualityScore >= 0.7 ? "#16a34a" : step.qualityScore >= 0.4 ? "#ca8a04" : "#dc2626",
                                                }}>
                                                  ★ {(step.qualityScore * 10).toFixed(0)}/10
                                                </span>
                                              )}
                                              <span style={{ color: "var(--text-muted)", fontSize: 10 }}>
                                                {new Date(step.createdAt).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}
                                              </span>
                                              <div style={{ marginLeft: "auto", display: "flex", gap: 6 }}>
                                                {!step.isCurrent && (
                                                  <button
                                                    style={{ background: "none", border: "none", cursor: "pointer", color: "var(--accent)", fontSize: 10 }}
                                                    disabled={promotingStep === step.id}
                                                    onClick={async () => {
                                                      setPromotingStep(step.id);
                                                      try {
                                                        const updated = await SetCurrentAssignmentStep(pa.id, step.id) as PersonAssignment;
                                                        setPersonAssignments(prev => prev.map(p => p.id === pa.id ? updated : p));
                                                        await loadSteps(pa.id);
                                                      } finally { setPromotingStep(null); }
                                                    }}
                                                  >
                                                    {promotingStep === step.id ? "Setting…" : "Set as current"}
                                                  </button>
                                                )}
                                                <button
                                                  style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", fontSize: 10 }}
                                                  onClick={() => { setEditStepContent(p => ({ ...p, [key]: p[key] ?? step.messageContent })); setEditingStepKey(editingStepKey === key ? null : key); }}
                                                >
                                                  Edit &amp; Rebuild
                                                </button>
                                              </div>
                                            </div>
                                            <div style={{ color: "var(--text-muted)", fontSize: 10, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: "100%", marginTop: 2 }}>
                                              {(step.messageContent || step.userInstruction || "").slice(0, 120)}
                                            </div>
                                            {editingStepKey === key && (
                                              <div style={{ marginTop: 6, display: "flex", flexDirection: "column", gap: 4 }}>
                                                <textarea
                                                  style={{ width: "100%", minHeight: 60, resize: "vertical", fontSize: 11, fontFamily: "inherit", textAlign: "left", padding: "6px 8px", boxSizing: "border-box", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, color: "var(--text)" }}
                                                  value={editStepContent[key] ?? step.messageContent}
                                                  onChange={e => setEditStepContent(p => ({ ...p, [key]: e.target.value }))}
                                                />
                                                <div style={{ display: "flex", gap: 4 }}>
                                                  <button
                                                    className="btn-primary"
                                                    style={{ fontSize: 10, padding: "2px 8px" }}
                                                    disabled={isRebuildingThis}
                                                    onClick={async () => {
                                                      setRebuildingFrom({ paId: pa.id, stepIndex: step.stepIndex });
                                                      try {
                                                        const updated = await RebuildAssignmentChainFrom(pa.id, step.stepIndex, editStepContent[key] ?? "") as PersonAssignment;
                                                        setPersonAssignments(prev => prev.map(p => p.id === pa.id ? updated : p));
                                                        await loadSteps(pa.id);
                                                        setEditingStepKey(null);
                                                      } finally { setRebuildingFrom(null); }
                                                    }}
                                                  >
                                                    {isRebuildingThis ? "Rebuilding…" : "Rebuild from here"}
                                                  </button>
                                                  <button className="btn-secondary" style={{ fontSize: 10, padding: "2px 6px" }} onClick={() => setEditingStepKey(null)}>Cancel</button>
                                                </div>
                                              </div>
                                            )}
                                          </div>
                                        );
                                      })}
                                    </div>
                                  )}
                                </div>
                              </>
                            )}
                          </div>
                        </div>
                      );
                    })()}

                    {/* Legacy bulk generate */}
                    <div>
                      <button
                        className="btn-secondary"
                        style={{ fontSize: 11, padding: "2px 8px", opacity: 0.7 }}
                        disabled={isGeneratingAssignments}
                        onClick={async () => {
                          setIsGeneratingAssignments(true);
                          try {
                            const pas = await GeneratePersonAssignments(id) as PersonAssignment[];
                            setPersonAssignments(pas ?? []);
                          } finally {
                            setIsGeneratingAssignments(false);
                          }
                        }}
                      >
                        {isGeneratingAssignments ? "Generating…" : "Bulk Generate All"}
                      </button>
                    </div>
                  </div>
                </details>
              </div>
            ) : (
              <div style={{ color: "var(--text-muted)", fontSize: 13 }}>No analysis yet. Click Analyze to start.</div>
            )}
          </div>
        </div>

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
                  {(() => {
                    const issueKeys = new Set(issues.map((i) => i.jiraKey));
                    const parents = issues.filter((i) => !i.parentKey || !issueKeys.has(i.parentKey));
                    const subtasksByParent = new Map<string, typeof issues>();
                    for (const i of issues) {
                      if (i.parentKey && issueKeys.has(i.parentKey)) {
                        const list = subtasksByParent.get(i.parentKey) ?? [];
                        list.push(i);
                        subtasksByParent.set(i.parentKey, list);
                      }
                    }
                    const rows: React.ReactNode[] = [];
                    const baseProps = {
                      onDeleteIssue: handleDeleteIssue,
                      onDeleteComment: handleDeleteComment,
                      onToggleIssue: (id: string) => void handleToggleIssue(id),
                      onToggleComment: (id: string) => void handleToggleComment(id),
                      onMarkIssueViewed: async (iid: string) => { await MarkIssueAllViewed(iid); load(true); void loadBreakdown(); },
                      onMarkCommentViewed: async (cid: string) => { await MarkCommentViewed(cid); load(true); void loadBreakdown(); },
                      jiraBaseUrl: detail?.jira?.baseUrl || jiraInstanceUrl,
                      onCheckoutBranch: (branchName: string, repoName: string) => {
                        const vb = vcsBranches.find((b) => b.repo?.name === repoName);
                        if (vb) void handleCheckout(branchName, vb.repoId);
                      },
                    };
                    for (const issue of parents) {
                      rows.push(<IssueCard key={issue.id} issue={issue} {...baseProps} />);
                      for (const sub of subtasksByParent.get(issue.jiraKey) ?? []) {
                        rows.push(<IssueCard key={sub.id} issue={sub} isSubtask {...baseProps} />);
                      }
                    }
                    return rows;
                  })()}
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
                      onMarkChannelAllViewed={async (cid) => { await MarkSlackChannelAllViewed(cid); load(true); void loadBreakdown(); }}
                    />
                  ))}

                  {/* Orphan threads (no matching configured channel, top-level only) */}
                  {slackThreads
                    .filter((t) => !t.parentThreadTs && !slackChannels.some((ch) => ch.channelId === t.channelId))
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
                    Branches and PRs
                    {breakdown && (breakdown.prs + breakdown.prComments + breakdown.notifications) > 0 && (
                      <span className="unread-badge" style={{ marginLeft: 6 }}>
                        {breakdown.prs + breakdown.prComments + breakdown.notifications} new
                      </span>
                    )}
                  </span>
                  <span className="detail-section-count">{pullRequests.length}</span>
                </div>
                {checkoutError && (
                  <div className="error-banner" style={{ margin: "0 14px 8px" }}>
                    {checkoutError}
                    <button
                      style={{ marginLeft: 8, background: "none", border: "none", cursor: "pointer", color: "inherit" }}
                      onClick={() => setCheckoutError("")}
                    >×</button>
                  </div>
                )}
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
                    const workspace = b.branch
                      ? workspaces.find((w) => w.branch === b.branch && w.repoId === b.repoId)
                      : undefined;
                    const checkoutNode = b.branch
                      ? workspace
                        ? (
                          <WorkspaceMenu
                            workspace={workspace}
                            checkoutLoading={checkoutLoading}
                            onCheckout={() => void handleCheckout(b.branch, b.repoId)}
                            onMarkDone={(id) => void handleMarkWorkspaceDone(id)}
                            onDelete={(id) => void handleDeleteWorkspace(id)}
                          />
                        ) : (
                          <button
                            type="button"
                            className="vcs-checkout-btn"
                            title={`Checkout ${b.branch} in workspace`}
                            disabled={checkoutLoading}
                            onClick={() => void handleCheckout(b.branch, b.repoId)}
                          >
                            Checkout
                          </button>
                        )
                      : undefined;
                    const hasUnread = branchPRs.some((pr) => !pr.viewedAt || (pr.comments ?? []).some((c) => !c.viewedAt));

                    return (
                      <div key={b.id} className="pr-repo-group">
                        {/* Repo / branch sub-header */}
                        <div className="pr-repo-group-header">
                          <GitBranch size={13} className="pr-repo-group-icon" />
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
                          {b.branch && (
                            branchUrl ? (
                              <button
                                type="button"
                                className="pr-branch-chip pr-branch-chip--link pr-branch-chip--header"
                                onClick={() => BrowserOpenURL(branchUrl)}
                                title={`Open branch ${b.branch} on GitHub`}
                              >
                                {b.branch}
                              </button>
                            ) : (
                              <span className="pr-branch-chip pr-branch-chip--header">{b.branch}</span>
                            )
                          )}
                          {b.label && (
                            <span className="vcs-label-badge" title="Repo label">{b.label}</span>
                          )}
                          {checkoutNode && (
                            <span
                              style={{ marginLeft: "auto", display: "inline-flex", alignItems: "center" }}
                              onClick={(e) => e.stopPropagation()}
                            >
                              {checkoutNode}
                            </span>
                          )}
                          {hasUnread && (
                            <button
                              type="button"
                              className="mark-viewed-btn"
                              style={{ marginLeft: checkoutNode ? 6 : "auto" }}
                              title="Mark all PRs and comments as viewed"
                              onClick={() => void (async () => { await MarkBranchAllViewed(b.id); load(true); void loadBreakdown(); })()}
                            ><Check size={13} /></button>
                          )}
                          {branchPRs.length > 0 && (
                            <span className="detail-section-count" style={{ marginLeft: checkoutNode || hasUnread ? 0 : "auto" }}>
                              {branchPRs.length}
                            </span>
                          )}
                        </div>

                        {/* PRs for this branch */}
                        {branchPRs.length === 0 ? (
                          <div className="detail-section-empty" style={{ padding: "8px 14px 12px" }}>
                            Click <strong>Refresh</strong> to fetch PRs
                          </div>
                        ) : branchPRs.map((pr) => (
                          <PRCard
                            key={pr.id}
                            pr={pr}
                            vcsBranches={vcsBranches}
                            onMarkPRViewed={async (pid) => { await MarkPRViewed(pid); load(true); void loadBreakdown(); }}
                            onMarkCommentViewed={async (cid) => { await MarkPRCommentViewed(cid); load(true); void loadBreakdown(); }}
                          />
                        ))}
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
                        {RELATION_TYPE_LABELS[rel.relationType as import("../types").ContextRelationType] ?? rel.relationType}
                      </span>
                      <button
                        type="button"
                        className="btn-ghost relation-link"
                        onClick={() => openContextTab(rel.targetId, rel.targetKey, rel.targetOrgName, rel.targetOrgId)}
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
              <button className="sidepanel-tab sidepanel-tab--active">
                Events
              </button>
            </div>
            <div className="sidepanel-body" style={{ overflowY: "auto", padding: "8px 12px", display: "flex", flexDirection: "column" }}>
              {isAnalyzing ? (
                <div className="event-skeleton-list">
                  {([0.85, 1.0, 0.6] as const).map((w) => (
                    <div key={w} className="event-skeleton-row">
                      <div className="event-skeleton-meta">
                        <div className="event-skeleton-line event-skeleton-line--date" />
                        <div className="event-skeleton-line event-skeleton-line--badge" />
                      </div>
                      <div className="event-skeleton-line event-skeleton-line--body" style={{ width: `${w * 100}%` }} />
                      <div className="event-skeleton-line event-skeleton-line--body" style={{ width: `${w * 55}%` }} />
                    </div>
                  ))}
                </div>
              ) : events.length === 0 ? (
                <div style={{ fontSize: 12, color: "var(--text-muted)", textAlign: "center", paddingTop: 24 }}>
                  No events
                </div>
              ) : events.map((ev) => (
                <div key={ev.id} style={{ fontSize: 12, borderBottom: "1px solid var(--border)", paddingBottom: 6, marginBottom: 6 }}>
                  {editingEventId === ev.id ? (
                    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                      <select
                        className="passphrase-input"
                        style={{ fontSize: 11, padding: "2px 4px" }}
                        value={editEventType}
                        onChange={(e) => setEditEventType(e.target.value)}
                      >
                        <option value="user_note">Note</option>
                        <option value="next_step_recommendation">Next step recommendation</option>
                        <option value="current_step_override">Correct current step</option>
                        <option value="open_question_answered">Answer open question</option>
                        <option value="requirement_confirmed">Confirm requirement</option>
                      </select>
                      <input
                        className="passphrase-input"
                        style={{ fontSize: 11, width: "100%" }}
                        value={editEventSummary}
                        onChange={(e) => setEditEventSummary(e.target.value)}
                        onKeyDown={async (e) => {
                          if (e.key === "Enter" && editEventSummary.trim()) {
                            await UpdateUserEvent(id, ev.id, editEventType, editEventSummary.trim());
                            setEditingEventId(null);
                            void loadSnapshot();
                          } else if (e.key === "Escape") {
                            setEditingEventId(null);
                          }
                        }}
                        autoFocus
                      />
                      <div style={{ display: "flex", gap: 4 }}>
                        <button
                          className="btn-primary"
                          style={{ fontSize: 11, padding: "2px 8px" }}
                          onClick={async () => {
                            if (!editEventSummary.trim()) return;
                            await UpdateUserEvent(id, ev.id, editEventType, editEventSummary.trim());
                            setEditingEventId(null);
                            void loadSnapshot();
                          }}
                        >
                          Save
                        </button>
                        <button
                          className="btn-secondary"
                          style={{ fontSize: 11, padding: "2px 8px" }}
                          onClick={() => setEditingEventId(null)}
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div style={{ display: "flex", gap: 6, alignItems: "center", marginBottom: 2 }}>
                        <span style={{ color: "var(--text-muted)", fontSize: 11 }}>
                          {(() => { try { return new Date(ev.timestamp).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }); } catch { return ev.timestamp; } })()}
                        </span>
                        <span className="platform-badge">{ev.sourceType}</span>
                        {ev.author && <span style={{ color: "var(--text-muted)" }}>{ev.author}</span>}
                        {ev.sourceType === "user" && (
                          <div style={{ display: "flex", gap: 2, marginLeft: "auto" }}>
                            <button
                              style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1 }}
                              title="Edit"
                              onClick={() => { setEditingEventId(ev.id); setEditEventType(ev.type); setEditEventSummary(ev.summary); }}
                            >
                              <Pencil size={11} />
                            </button>
                            <button
                              style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1 }}
                              title="Delete"
                              onClick={async () => { await DeleteUserEvent(id, ev.id); void loadSnapshot(); }}
                            >
                              <Trash2 size={11} />
                            </button>
                          </div>
                        )}
                      </div>
                      <div>{ev.summary}</div>
                    </>
                  )}
                </div>
              ))}
              {!isAnalyzing && (
                showAddForm ? (
                  <div style={{ borderTop: "1px solid var(--border)", paddingTop: 8, marginTop: 4 }}>
                    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                      <select
                        className="passphrase-input"
                        style={{ fontSize: 12, padding: "2px 6px" }}
                        value={addEventType}
                        onChange={(e) => setAddEventType(e.target.value)}
                      >
                        <option value="user_note">Note</option>
                        <option value="next_step_recommendation">Next step recommendation</option>
                        <option value="current_step_override">Correct current step</option>
                        <option value="open_question_answered">Answer open question</option>
                        <option value="requirement_confirmed">Confirm requirement</option>
                      </select>
                      <input
                        className="passphrase-input"
                        style={{ fontSize: 12, width: "100%" }}
                        placeholder="Summary…"
                        value={addEventSummary}
                        onChange={(e) => setAddEventSummary(e.target.value)}
                        onKeyDown={async (e) => {
                          if (e.key === "Enter" && addEventSummary.trim()) {
                            await AddUserEvent(id, addEventType, addEventSummary.trim());
                            setAddEventSummary("");
                            setShowAddForm(false);
                            void loadSnapshot();
                          } else if (e.key === "Escape") {
                            setShowAddForm(false);
                            setAddEventSummary("");
                          }
                        }}
                        autoFocus
                      />
                      <div style={{ display: "flex", gap: 4 }}>
                        <button
                          className="btn-primary"
                          style={{ fontSize: 12, padding: "2px 10px", flex: 1 }}
                          onClick={async () => {
                            if (!addEventSummary.trim()) return;
                            await AddUserEvent(id, addEventType, addEventSummary.trim());
                            setAddEventSummary("");
                            setShowAddForm(false);
                            void loadSnapshot();
                          }}
                        >
                          Add
                        </button>
                        <button
                          className="btn-secondary"
                          style={{ fontSize: 12, padding: "2px 10px" }}
                          onClick={() => { setShowAddForm(false); setAddEventSummary(""); }}
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  </div>
                ) : (
                  <button
                    style={{ width: "100%", background: "none", border: "1px dashed var(--border)", borderRadius: 6, cursor: "pointer", padding: "6px 0", fontSize: 16, color: "var(--text-muted)", marginTop: 4 }}
                    onClick={() => setShowAddForm(true)}
                  >
                    +
                  </button>
                )
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
      {showAuthorModal && detail && (
        <UnresolvedAuthorsModal
          contextID={id}
          orgID={detail.context.orgId}
          onClose={() => setShowAuthorModal(false)}
        />
      )}
      {checkoutTarget && checkoutPaths.length > 1 && (
        <div className="modal-overlay">
          <div className="modal">
            <h2>Select Local Path</h2>
            <p className="confirm-msg">
              Choose which local clone to use for checking out <strong>{checkoutTarget.branch}</strong>:
            </p>
            <div style={{ display: "flex", flexDirection: "column", gap: 8, margin: "12px 0" }}>
              {checkoutLoading ? (
                <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 0", fontSize: 12, color: "var(--text-muted)" }}>
                  <Spinner size={14} /> Checking out…
                </div>
              ) : checkoutPaths.map((p) => (
                <button
                  key={p.id}
                  className="btn-secondary"
                  style={{ textAlign: "left", padding: "8px 12px", fontSize: 12 }}
                  onClick={() => void handleCheckoutWithPath(p.id)}
                >
                  {p.path}
                </button>
              ))}
            </div>
            {checkoutError && <div className="error-banner">{checkoutError}</div>}
            <div className="modal-footer">
              <button
                className="btn-secondary"
                onClick={dismissCheckoutModal}
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
      {reasoningGraphFor && (() => {
        const pa = personAssignments.find(p => p.id === reasoningGraphFor);
        if (!pa) return null;
        return (
          <ReasoningChainGraph
            personAssignment={pa}
            steps={stepsMap[pa.id] ?? []}
            onClose={() => setReasoningGraphFor(null)}
            onStepsChanged={(updated, newSteps) => {
              setPersonAssignments(prev => prev.map(p => p.id === updated.id ? updated : p));
              setStepsMap(prev => ({ ...prev, [updated.id]: newSteps }));
            }}
          />
        );
      })()}
    </div>
  );
}
