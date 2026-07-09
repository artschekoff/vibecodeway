import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import Spinner from "../components/Spinner";
import { useTabContext } from "../contexts/TabContext";
import { RefreshCw, RotateCcw, Settings, Pencil, X, ExternalLink, GitBranch, Hash, FileText, GripVertical, Folder, FolderOpen, Check, Copy, Trash2, Loader2 } from "lucide-react";
import {
  DndContext,
  closestCenter,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
  arrayMove,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  ListContextsByOrg,
  CreateContext,
  UpdateContext,
  DeleteContext,
  AuthStatus,
  SetJiraConfig,
  AddIssuesToContext,
  AddSlackChannels,
  RemoveSlackChannel,
  GetContextDetail,
  ReloadAllContexts,
  RefreshAllContexts,
  UpsertContextText,
  DeleteContextText,
  DeleteUploadedFile,
  ToggleContextStatus,
  UpsertSlackDMConfig,
  DeleteSlackDMConfig,
  ListSlackDMConfigs,
  CommitPendingUploads,
  SearchContextIDs,
  GetPendingCodeReviews,
  RefreshPendingReviews,
  ListTrackedRepos,
  AddTrackedRepo,
  RemoveTrackedRepo,
  UpdateTrackedRepoMeta,
  GetSupportedPlatforms,
  SetContextsOrder,
  PickRepoDirectory,
  SetTrackedRepoPath,
  ListRepoLocalPaths,
  AddRepoLocalPath,
  RemoveRepoLocalPath,
  DismissReviewRequest,
  OpenWorkspaceTerminal,
  OpenRepoPathTerminal,
  ListWorkspaces,
  RefreshWorkspacesForRepo,
  DeleteWorkspace,
  MarkWorkspaceShouldDelete,
  ListLocalBranches,
  DeleteLocalBranch,
  CheckoutLocalBranch,
  SwitchMainWorkingTree,
  FetchPruneRepoPath,
  ListOrganizations,
  SetContextOrg,
  // @ts-ignore
  AddContextRelation,
  // @ts-ignore
  RemoveContextRelation,
  ListWorkflowStatuses,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff, BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type {
  ContextText, Context, Issue, SlackChannel, SlackDMConfig,
  SlackSavePayload, TextFilesPayload, UploadedFile,
  ReviewRequest, VCSRepo, VCSPlatform, VCSPlatformInfo,
  ContextRelation, RelationsPayload, Organization, VCSRepoPath, VCSWorkspace, LocalBranch,
  WorkflowStatus,
} from "../types";
import { effectiveStatus } from "../types";
import type { VCSBranchInput } from "../components/ContextForm";
import type { service } from "../../wailsjs/go/models";
import ContextForm from "../components/ContextForm";
import WorkflowStatusBadge from "../components/WorkflowStatusBadge";
import { useProgress } from "../contexts/ProgressContext";
import { usePassphrase } from "../contexts/PassphraseContext";
import { useCheckout } from "../lib/useCheckout";
import CheckoutLogModal from "../components/CheckoutLogModal";

// ─── Reviews helpers ──────────────────────────────────────────────────────────

interface RepoGroup { repo: VCSRepo; reviews: ReviewRequest[] }

function repoUrl(repo: VCSRepo): string {
  if (repo.platform === "github") return `https://github.com/${repo.owner}/${repo.name}`;
  if (repo.platform === "gitlab") return `https://gitlab.com/${repo.owner}/${repo.name}`;
  return "";
}

function ReviewCard({
  review,
  onDismiss,
  onCheckout,
  checkoutLoading,
}: {
  review: ReviewRequest;
  onDismiss?: (id: string) => void;
  onCheckout?: () => void;
  checkoutLoading?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const isWaiting = !!review.reviewedAt;

  const handleCopy = () => {
    if (!review.prUrl) return;
    void navigator.clipboard.writeText(review.prUrl).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  const statusClass =
    review.reviewStatus === "changes_requested" ? " pr-status--changes-requested"
    : review.reviewStatus === "approved" ? " pr-status--approved"
    : "";
  const statusLabel =
    review.reviewStatus === "changes_requested" ? "Changes Requested"
    : review.reviewStatus === "approved" ? "Approved"
    : review.reviewStatus === "review_required" ? "Review Required"
    : null;

  return (
    <div className={`review-card${isWaiting ? " review-card--waiting" : ""}`}>
      <div className="review-card-top">
        <span className="review-card-number">#{review.prNumber}</span>
        <span className="review-card-title">{review.prTitle || "(no title)"}</span>
        {isWaiting
          ? <span className="pr-status-badge pr-status--waiting">Waiting for Author</span>
          : statusLabel && <span className={`pr-status-badge${statusClass}`}>{statusLabel}</span>
        }
      </div>
      <div className="review-card-bottom">
        <span className="review-card-author">by {review.prAuthor || "unknown"}</span>
        <div className="review-card-btns">
          {review.headBranch && onCheckout && (
            <button
              className="review-open-btn"
              onClick={onCheckout}
              disabled={checkoutLoading}
              title={`Checkout ${review.headBranch} in workspace`}
            >
              <GitBranch size={11} /> Checkout
            </button>
          )}
          {review.prUrl && (
            <button className="review-open-btn" onClick={handleCopy} title="Copy PR link">
              {copied ? <Check size={11} /> : <Copy size={11} />}
            </button>
          )}
          {review.prUrl && (
            <button className="review-open-btn" onClick={() => BrowserOpenURL(review.prUrl)} title="Open PR">
              Open <ExternalLink size={11} />
            </button>
          )}
          {isWaiting && onDismiss && (
            <button className="review-open-btn review-dismiss-btn" onClick={() => onDismiss(review.id)} title="Dismiss">
              <X size={11} />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── Sortable row ─────────────────────────────────────────────────────────────

interface SortableRowProps {
  id: string;
  disabled: boolean;
  className: string;
  onClick: () => void;
  children: (dragHandleProps: React.HTMLAttributes<HTMLElement>) => React.ReactNode;
}

function SortableContextRow({ id, disabled, className, onClick, children }: SortableRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id, disabled });
  return (
    <tr
      ref={setNodeRef}
      className={className}
      onClick={onClick}
      style={{
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.45 : 1,
        position: isDragging ? "relative" : undefined,
        zIndex: isDragging ? 10 : undefined,
      }}
    >
      {children({ ...listeners, ...attributes })}
    </tr>
  );
}

// ─── Main component ───────────────────────────────────────────────────────────

const PROVIDER_LABELS: Record<string, string> = {
  jira: "Fetching Jira issues",
  slack: "Fetching Slack messages",
  vcs: "Fetching pull requests",
  text: "Syncing texts & files",
};

type HomeTab = "contexts" | "reviews" | "repositories";

interface ContextListProps {
  orgId: string;
  tabId: string;
}

export default function ContextList({ orgId, tabId: _tabId }: ContextListProps) {
  const { openContextTab, openNewContextTab, openSettingsTab, tabs, forceCloseTab } = useTabContext();
  const progress = useProgress();
  const { ensurePassphrase } = usePassphrase();

  // ── Tab state (persisted) ──
  const [activeTab, setActiveTab] = useState<HomeTab>(() => {
    const saved = localStorage.getItem("home-tab");
    if (saved === "reviews" || saved === "repositories") return saved;
    return "contexts";
  });

  const switchTab = (tab: HomeTab) => {
    setActiveTab(tab);
    localStorage.setItem("home-tab", tab);
  };

  // ── Contexts state ──
  const [contexts, setContexts] = useState<Context[]>([]);
  const [loading, setLoading] = useState(true);
  const [authWarning, setAuthWarning] = useState(false);
  const [checkingAuth, setCheckingAuth] = useState(false);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [editTarget, setEditTarget] = useState<Context | null>(null);
  const [editJqlQuery, setEditJqlQuery] = useState<string | undefined>(undefined);
  const [editJiraIssues, setEditJiraIssues] = useState<Issue[] | undefined>(undefined);
  const [editSlackChannels, setEditSlackChannels] = useState<SlackChannel[] | undefined>(undefined);
  const [editSlackDMConfigs, setEditSlackDMConfigs] = useState<SlackDMConfig[] | undefined>(undefined);
  const [editContextTexts, setEditContextTexts] = useState<ContextText[] | undefined>(undefined);
  const [editUploadedFiles, setEditUploadedFiles] = useState<UploadedFile[] | undefined>(undefined);
  const [editRelations, setEditRelations] = useState<ContextRelation[] | undefined>(undefined);
  const [deleteTarget, setDeleteTarget] = useState<Context | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [reloadingAll, setReloadingAll] = useState(false);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [reloadResults, setReloadResults] = useState<service.ReloadAllResult[] | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [matchedIDs, setMatchedIDs] = useState<Set<string> | null>(null);
  const searchTimer = useRef<ReturnType<typeof setTimeout>>();
  const initialFetchPruneDone = useRef(false);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [workflowStatuses, setWorkflowStatuses] = useState<WorkflowStatus[]>([]);

  // ── Reviews state ──
  const [repoGroups, setRepoGroups] = useState<RepoGroup[]>([]);
  const [trackedRepos, setTrackedRepos] = useState<VCSRepo[]>([]);
  const [platforms, setPlatforms] = useState<VCSPlatformInfo[]>([]);
  const [totalReviews, setTotalReviews] = useState(0);
  const [refreshingReviews, setRefreshingReviews] = useState(false);
  const [reviewFilter, setReviewFilter] = useState("");
  const [showAddRepo, setShowAddRepo] = useState(false);
  const [addPlatform, setAddPlatform] = useState<VCSPlatform>("github");
  const [addOwnerRepo, setAddOwnerRepo] = useState("");
  const [addRepoError, setAddRepoError] = useState("");
  const [addingRepo, setAddingRepo] = useState(false);

  // ── Repositories edit state ──
  const [editingRepoId, setEditingRepoId] = useState<string | null>(null);
  const [repoPaths, setRepoPaths] = useState<Record<string, VCSRepoPath[]>>({});
  const [repoWorkspaces, setRepoWorkspaces] = useState<Record<string, VCSWorkspace[]>>({});
  const [localBranches, setLocalBranches] = useState<Record<string, LocalBranch[]>>({});
  const [refreshingRepoId, setRefreshingRepoId] = useState<string | null>(null);
  const [editLabel, setEditLabel] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editOnCheckoutCommand, setEditOnCheckoutCommand] = useState("");
  const [editSaving, setEditSaving] = useState(false);
  const [editError, setEditError] = useState("");

  // ── Code Reviews checkout ──
  const {
    checkout: handleReviewCheckout,
    checkoutWithPath: handleReviewCheckoutWithPath,
    dismissCheckoutModal: dismissReviewCheckoutModal,
    checkoutTarget: reviewCheckoutTarget,
    checkoutPaths: reviewCheckoutPaths,
    checkoutLoading: reviewCheckoutLoading,
    checkoutError: reviewCheckoutError,
    setCheckoutError: setReviewCheckoutError,
    logs: reviewLogs,
    logRunning: reviewLogRunning,
    logExitCode: reviewLogExitCode,
    logCommand: reviewLogCommand,
    logOpen: reviewLogOpen,
    dismissLog: dismissReviewLog,
  } = useCheckout(() => {}, "No local path set for this repository. Add one in the Repositories tab.");

  // ── DnD ──
  const isDraggingEnabled = !matchedIDs; // disable while searching
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    setContexts((prev) => {
      const oldIdx = prev.findIndex((c) => c.id === active.id);
      const newIdx = prev.findIndex((c) => c.id === over.id);
      const reordered = arrayMove(prev, oldIdx, newIdx);
      void SetContextsOrder(reordered.map((c) => c.id));
      return reordered;
    });
  }

  // ── Computed ──
  const totalUnviewed = useMemo(
    () => contexts.reduce((sum, c) => sum + (c.unviewedCount ?? 0), 0),
    [contexts],
  );

  const filteredContexts = useMemo(
    () => matchedIDs ? contexts.filter((c) => matchedIDs.has(c.id)) : contexts,
    [contexts, matchedIDs],
  );

  const needsAttentionContexts = useMemo(
    () => contexts.filter((c) => effectiveStatus(c) === "needs_attention"),
    [contexts],
  );

  // ── Load contexts ──
  const load = useCallback(async () => {
    try {
      const list = await ListContextsByOrg(orgId);
      setContexts(list ?? []);
    } catch {
      // Wails bridge not available or load failed
    } finally {
      setLoading(false);
    }
  }, [orgId]);

  // ── Load reviews ──
  const loadReviews = useCallback(async () => {
    try {
      const [reviews, repos, plats] = await Promise.all([
        GetPendingCodeReviews() as Promise<ReviewRequest[]>,
        ListTrackedRepos("") as Promise<VCSRepo[]>,
        GetSupportedPlatforms() as Promise<VCSPlatformInfo[]>,
      ]);
      const safeRepos = repos ?? [];
      const safeReviews = reviews ?? [];
      setTrackedRepos(safeRepos);
      setPlatforms(plats ?? []);
      setRepoGroups(safeRepos.map((repo) => ({
        repo,
        reviews: safeReviews.filter((r) => r.repoId === repo.id),
      })));
      setTotalReviews(safeReviews.filter((r) => !r.reviewedAt).length);
      // Fetch local paths for each repo
      const pathResults = await Promise.allSettled(
        safeRepos.map((repo) =>
          (ListRepoLocalPaths(repo.id) as Promise<VCSRepoPath[]>).then(
            (paths) => ({ id: repo.id, paths: paths ?? [] })
          )
        )
      );
      const pathMap: Record<string, VCSRepoPath[]> = {};
      for (const r of pathResults) {
        if (r.status === "fulfilled") pathMap[r.value.id] = r.value.paths;
      }
      setRepoPaths(pathMap);
      // Fetch workspaces and local branches for each path
      const wsMap: Record<string, VCSWorkspace[]> = {};
      const brMap: Record<string, LocalBranch[]> = {};
      for (const r of pathResults) {
        if (r.status === "fulfilled") {
          for (const p of r.value.paths) {
            wsMap[p.id] = (await ListWorkspaces(p.id) as VCSWorkspace[]) ?? [];
            brMap[p.id] = await (ListLocalBranches(p.id) as Promise<LocalBranch[]>).catch(() => []);
          }
        }
      }
      setRepoWorkspaces(wsMap);
      setLocalBranches(brMap);
    } catch {
      // best-effort
    }
  }, []);

  const checkAuth = useCallback(async () => {
    setCheckingAuth(true);
    try {
      const s = await AuthStatus();
      setAuthWarning(s.toLowerCase().includes("not logged") || s.toLowerCase().includes("error"));
    } catch {
      // Wails bridge not available — ignore
    } finally {
      setCheckingAuth(false);
    }
  }, []);

  useEffect(() => {
    load();
    void checkAuth();
    void loadReviews();
  }, [load, checkAuth, loadReviews]);

  useEffect(() => {
    ListOrganizations().then((list: Organization[]) => setOrgs(list ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    void ListWorkflowStatuses().then((ss) => setWorkflowStatuses(ss ?? []));
  }, []);

  // ── Background refresh event listeners ──
  useEffect(() => {
    const offContexts = EventsOn("bg-refresh:contexts", () => { void load(); });
    const offReviews = EventsOn("bg-refresh:reviews", () => { void loadReviews(); });
    return () => {
      offContexts();
      offReviews();
      EventsOff("bg-refresh:contexts");
      EventsOff("bg-refresh:reviews");
    };
  }, [load, loadReviews]);

  // On first load, background fetch-prune all repo paths to get accurate "gone" status.
  useEffect(() => {
    const paths = Object.values(repoPaths).flat();
    if (paths.length === 0 || initialFetchPruneDone.current) return;
    initialFetchPruneDone.current = true;
    paths.forEach((p) => {
      (FetchPruneRepoPath(p.id) as Promise<void>)
        .then(() => (ListLocalBranches(p.id) as Promise<LocalBranch[]>).catch(() => []))
        .then((branches) => setLocalBranches((prev) => ({ ...prev, [p.id]: branches })))
        .catch(() => {});
    });
  }, [repoPaths]);

  // ── Search ──
  const handleSearchChange = useCallback((value: string) => {
    setSearchQuery(value);
    clearTimeout(searchTimer.current);
    if (!value.trim()) { setMatchedIDs(null); return; }
    searchTimer.current = setTimeout(async () => {
      try {
        const ids = await SearchContextIDs(value.trim());
        setMatchedIDs(new Set(ids ?? []));
      } catch {
        setMatchedIDs(null);
      }
    }, 200);
  }, []);

  // ── Context CRUD ──
  const handleCreate = async (
    key: string, searchQuery: string, _status: string | undefined,
    slackPayload?: SlackSavePayload, selectedJiraKeys?: string[],
    _removedJiraIssueIds?: string[], textFilesPayload?: TextFilesPayload,
    _removedUploadedFileIds?: string[], pendingUploadTempIds?: string[],
    _newVCSBranches?: VCSBranchInput[], _removedVCSBranchIds?: string[],
    relationsPayload?: RelationsPayload
  ) => {
    const jiraKeys = selectedJiraKeys ?? [];
    const slackGroups = (slackPayload?.groups ?? []).filter((g) => g.channels.length > 0);
    const dmConfigs = slackPayload?.dmConfigs ?? [];
    const newTextFiles = textFilesPayload?.newFiles ?? [];
    const pendingIds = pendingUploadTempIds ?? [];
    const itemLabels = [
      ...jiraKeys,
      ...slackGroups.flatMap((g) => g.channels.map((ch) => `#${ch.channelName}  ·  ${g.workspace}`)),
      ...dmConfigs.map((dm) => `DMs from ${dm.workspace}`),
      ...newTextFiles.map((f) => f.name || "Text file"),
      ...pendingIds.map(() => "Uploading file"),
    ];
    progress.begin("Creating Context", itemLabels);
    try {
      const ctx = await CreateContext(key, orgId);
      if (searchQuery.trim()) await SetJiraConfig(ctx.id, searchQuery.trim());
      for (const jiraKey of jiraKeys) {
        progress.next(`Adding issue ${jiraKey}`);
        try { await AddIssuesToContext(ctx.id, [jiraKey]); } catch { /* best-effort */ }
      }
      for (const group of slackGroups) {
        progress.next(`Adding ${group.channels.map((ch) => `#${ch.channelName}`).join(", ")} from ${group.workspace}`);
        try { await AddSlackChannels(ctx.id, group.workspace, group.channels as any); } catch { /* best-effort */ }
      }
      if (dmConfigs.length > 0 && !(await ensurePassphrase())) {
        // user cancelled passphrase — skip DM upserts
      } else {
        for (const dm of dmConfigs) {
          progress.next(`Loading DMs from ${dm.workspace}`);
          try { await UpsertSlackDMConfig(ctx.id, dm.workspace, dm.startFrom); } catch { /* best-effort */ }
        }
      }
      for (const f of newTextFiles) {
        progress.next(`Saving ${f.name || "text file"}`);
        try { await UpsertContextText("", ctx.id, f.name, f.content); } catch { /* best-effort */ }
      }
      if (pendingIds.length > 0) {
        progress.next("Saving uploaded files");
        try { await CommitPendingUploads(ctx.id, pendingIds); } catch { /* best-effort */ }
      }
      for (const rel of (relationsPayload?.newRelations ?? [])) {
        try { await AddContextRelation(ctx.id, rel.targetId, rel.relationType); } catch { /* best-effort */ }
      }
    } finally { progress.done(); }
    setShowForm(false);
    load();
  };

  const openEditForm = async (c: Context) => {
    let jql = "", issues: Issue[] = [], channels: SlackChannel[] = [],
      dmConfigs: SlackDMConfig[] = [], contextTexts: ContextText[] = [],
      uploadedFiles: UploadedFile[] = [], relations: ContextRelation[] = [];
    try {
      const [detail, dms] = await Promise.all([GetContextDetail(c.id), ListSlackDMConfigs(c.id)]);
      jql = detail.jira?.searchQuery ?? "";
      issues = detail.issues ?? [];
      channels = detail.slackChannels ?? [];
      contextTexts = detail.contextTexts ?? [];
      uploadedFiles = (detail.uploadedFiles ?? []) as UploadedFile[];
      relations = (detail.relations ?? []) as ContextRelation[];
      dmConfigs = dms ?? [];
    } catch { /* best-effort */ }
    setEditJqlQuery(jql);
    setEditJiraIssues(issues);
    setEditSlackChannels(channels);
    setEditSlackDMConfigs(dmConfigs);
    setEditContextTexts(contextTexts);
    setEditUploadedFiles(uploadedFiles);
    setEditRelations(relations);
    setEditTarget(c);
  };

  const closeEditForm = () => {
    setEditTarget(null);
    setEditJqlQuery(undefined);
    setEditJiraIssues(undefined);
    setEditSlackChannels(undefined);
    setEditSlackDMConfigs(undefined);
    setEditContextTexts(undefined);
    setEditUploadedFiles(undefined);
    setEditRelations(undefined);
  };

  const handleUpdate = async (
    key: string, searchQuery: string, status: string | undefined,
    slackPayload?: SlackSavePayload, selectedJiraKeys?: string[],
    _removedJiraIssueIds?: string[], textFilesPayload?: TextFilesPayload,
    removedUploadedFileIds?: string[],
    _pendingUploadTempIds?: string[],
    _newVCSBranches?: VCSBranchInput[],
    _removedVCSBranchIds?: string[],
    relationsPayload?: RelationsPayload
  ) => {
    if (!editTarget) return;
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
      ...slackGroups.flatMap((g) => g.channels.map((ch) => `#${ch.channelName}  ·  ${g.workspace}`)),
      ...dmConfigs.map((dm) => `DMs from ${dm.workspace}`),
      ...newTextFiles.map((f) => f.name || "Text file"),
    ];
    progress.begin("Updating Context", itemLabels);
    try {
      await UpdateContext(editTarget.id, key, status ?? editTarget.status);
      if (searchQuery.trim()) await SetJiraConfig(editTarget.id, searchQuery.trim());
      for (const jiraKey of jiraKeys) {
        progress.next(`Adding issue ${jiraKey}`);
        try { await AddIssuesToContext(editTarget.id, [jiraKey]); } catch { /* best-effort */ }
      }
      for (const id of removedChannelIds) { try { await RemoveSlackChannel(id); } catch { /* best-effort */ } }
      for (const group of slackGroups) {
        progress.next(`Adding ${group.channels.map((ch) => `#${ch.channelName}`).join(", ")} from ${group.workspace}`);
        try { await AddSlackChannels(editTarget.id, group.workspace, group.channels as any); } catch { /* best-effort */ }
      }
      for (const id of removedDMConfigIds) { try { await DeleteSlackDMConfig(id); } catch { /* best-effort */ } }
      if (dmConfigs.length > 0 && !(await ensurePassphrase())) {
        // user cancelled passphrase — skip DM upserts
      } else {
        for (const dm of dmConfigs) {
          progress.next(`Loading DMs from ${dm.workspace}`);
          try { await UpsertSlackDMConfig(editTarget.id, dm.workspace, dm.startFrom); } catch { /* best-effort */ }
        }
      }
      for (const id of removedTextFileIds) { try { await DeleteContextText(id); } catch { /* best-effort */ } }
      for (const f of editedTextFiles) { try { await UpsertContextText(f.id, editTarget.id, f.name, f.content); } catch { /* best-effort */ } }
      for (const f of newTextFiles) {
        progress.next(`Saving ${f.name || "text file"}`);
        try { await UpsertContextText("", editTarget.id, f.name, f.content); } catch { /* best-effort */ }
      }
      for (const fId of (removedUploadedFileIds ?? [])) { try { await DeleteUploadedFile(fId); } catch { /* best-effort */ } }
      for (const relId of (relationsPayload?.removedRelationIds ?? [])) {
        try { await RemoveContextRelation(relId); } catch { /* best-effort */ }
      }
      for (const rel of (relationsPayload?.newRelations ?? [])) {
        try { await AddContextRelation(editTarget.id, rel.targetId, rel.relationType); } catch { /* best-effort */ }
      }
    } finally { progress.done(); }
    closeEditForm();
    load();
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    const deletedId = deleteTarget.id;
    try {
      await DeleteContext(deletedId);
      setDeleteTarget(null);
      // Close the open tab before reloading, so it's removed regardless of reload result
      const openTab = tabs.find((t) => t.type === "context" && t.contextId === deletedId);
      if (openTab) forceCloseTab(openTab.id);
      void load();
    } finally {
      setDeleting(false);
    }
  };

  const handleReloadAll = async () => {
    setReloadingAll(true);
    progress.begin("Reloading All Contexts", []);
    const unsubscribe = EventsOn(
      "reload:progress",
      (data: { current: number; total: number; contextKey: string; step: string }) => {
        const stepLabel = PROVIDER_LABELS[data.step] ?? data.step;
        progress.set(data.current - 1, data.total, data.contextKey, stepLabel);
      }
    );
    try {
      const results = await ReloadAllContexts();
      setReloadResults(results ?? []);
      load();
    } catch { setError("Reload failed — check logs for details."); }
    finally { unsubscribe(); EventsOff("reload:progress"); setReloadingAll(false); progress.done(); }
  };

  const handleRefreshAll = async () => {
    setRefreshingAll(true);
    progress.begin("Refreshing All Contexts", []);
    const unsubscribe = EventsOn(
      "reload:progress",
      (data: { current: number; total: number; contextKey: string; step: string }) => {
        const stepLabel = PROVIDER_LABELS[data.step] ?? data.step;
        progress.set(data.current - 1, data.total, data.contextKey, stepLabel);
      }
    );
    try { await RefreshAllContexts(); load(); }
    catch { setError("Refresh failed — check logs for details."); }
    finally { unsubscribe(); EventsOff("reload:progress"); setRefreshingAll(false); progress.done(); }
  };

  const handleToggleStatus = async (_e: React.MouseEvent, c: Context) => {
    try {
      const updated = await ToggleContextStatus(c.id);
      setContexts((prev) => prev.map((ctx) => ctx.id === updated.id ? { ...ctx, status: updated.status } : ctx));
    } catch { /* best-effort */ }
  };

  // ── Reviews handlers ──
  const handleRefreshReviews = async () => {
    setRefreshingReviews(true);
    try { await RefreshPendingReviews(); await loadReviews(); }
    catch (e) { setError(String(e)); }
    finally { setRefreshingReviews(false); }
  };

  const handleDismissReview = async (id: string) => {
    try { await DismissReviewRequest(id); await loadReviews(); }
    catch (e) { setError(String(e)); }
  };

  const handleAddRepo = async () => {
    const trimmed = addOwnerRepo.trim();
    if (!trimmed || !trimmed.includes("/")) { setAddRepoError("Enter owner/repo (e.g. octocat/hello-world)"); return; }
    const [owner, ...rest] = trimmed.split("/");
    const name = rest.join("/");
    if (!owner || !name) { setAddRepoError("Enter owner/repo"); return; }
    setAddingRepo(true);
    setAddRepoError("");
    try {
      await AddTrackedRepo(addPlatform, `${owner}/${name}`);
      setAddOwnerRepo("");
      setShowAddRepo(false);
      await loadReviews();
    } catch (e) { setAddRepoError(String(e)); }
    finally { setAddingRepo(false); }
  };

  const handleRemoveRepo = async (repoId: string) => {
    try {
      await RemoveTrackedRepo(repoId);
      if (editingRepoId === repoId) setEditingRepoId(null);
      await loadReviews();
    } catch (e) { setError(String(e)); }
  };

  const handleSetRepoPath = async (repoId: string) => {
    try {
      const path = await PickRepoDirectory();
      if (!path) return;
      await SetTrackedRepoPath(repoId, path);
      await loadReviews();
    } catch (e) { setError(String(e)); }
  };

  const handleAddRepoPath = async (repoId: string) => {
    try {
      await AddRepoLocalPath(repoId);
      await loadReviews();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleRemoveRepoPath = async (pathId: string) => {
    try {
      await RemoveRepoLocalPath(pathId);
      await loadReviews();
    } catch (e) {
      setError(String(e));
    }
  };

  const loadWorkspacesForRepo = async (repoId: string, fetchPrune = false) => {
    const paths = (await ListRepoLocalPaths(repoId) as VCSRepoPath[]) ?? [];
    const wsResult: Record<string, VCSWorkspace[]> = {};
    const brResult: Record<string, LocalBranch[]> = {};
    await Promise.all(
      paths.map(async (p) => {
        if (fetchPrune) await (FetchPruneRepoPath(p.id) as Promise<void>).catch(() => {});
        wsResult[p.id] = (await ListWorkspaces(p.id) as VCSWorkspace[]) ?? [];
        brResult[p.id] = await (ListLocalBranches(p.id) as Promise<LocalBranch[]>).catch(() => []);
      })
    );
    setRepoWorkspaces((prev) => ({ ...prev, ...wsResult }));
    setLocalBranches((prev) => ({ ...prev, ...brResult }));
  };

  const reloadLocalBranches = async (pathId: string) => {
    const branches = await (ListLocalBranches(pathId) as Promise<LocalBranch[]>).catch(() => []);
    setLocalBranches((prev) => ({ ...prev, [pathId]: branches }));
  };

  const handleRefreshWorkspaces = async (repoId: string) => {
    setRefreshingRepoId(repoId);
    try {
      await RefreshWorkspacesForRepo(repoId);
      await loadWorkspacesForRepo(repoId, true);
    } catch (e) {
      setError(String(e));
    } finally {
      setRefreshingRepoId(null);
    }
  };

  const handleDeleteWorkspace = async (workspaceId: string, repoId: string) => {
    try {
      await DeleteWorkspace(workspaceId);
      await loadWorkspacesForRepo(repoId);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleMarkWorkspaceDone = async (workspaceId: string, repoId: string) => {
    try {
      await MarkWorkspaceShouldDelete(workspaceId);
      await loadWorkspacesForRepo(repoId);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDeleteLocalBranch = async (pathId: string, branch: string) => {
    try {
      await DeleteLocalBranch(pathId, branch);
      await reloadLocalBranches(pathId);
    } catch (e) {
      setError(String(e));
    }
  };

  const [checkingOutKey, setCheckingOutKey] = useState<string | null>(null);
  const [openingWorkspaceId, setOpeningWorkspaceId] = useState<string | null>(null);

  const handleCheckoutLocalBranch = async (pathId: string, branch: string, repoId: string) => {
    const key = `${pathId}:${branch}`;
    if (checkingOutKey) return;
    setCheckingOutKey(key);
    try {
      await CheckoutLocalBranch(pathId, branch);
      await loadWorkspacesForRepo(repoId);
    } catch (e) {
      setError(String(e));
    } finally {
      setCheckingOutKey(null);
    }
  };

  const handleSwitchMainWorkingTree = async (pathId: string, branch: string) => {
    try {
      await SwitchMainWorkingTree(pathId, branch);
      await reloadLocalBranches(pathId);
    } catch (e) {
      setError(String(e));
    }
  };

  const handleUpdateRepoMeta = async (repoId: string) => {
    setEditSaving(true);
    setEditError("");
    try {
      await UpdateTrackedRepoMeta(repoId, editLabel.trim(), editDescription.trim(), editOnCheckoutCommand.trim());
      setEditingRepoId(null);
      await loadReviews();
    } catch (e) {
      setEditError(String(e));
    } finally {
      setEditSaving(false);
    }
  };

  const startEdit = (repo: VCSRepo) => {
    setEditingRepoId(repo.id);
    setEditLabel(repo.label ?? "");
    setEditDescription(repo.description ?? "");
    setEditOnCheckoutCommand(repo.onCheckoutCommand ?? "");
    setEditError("");
  };

  const cancelEdit = () => {
    setEditingRepoId(null);
    setEditError("");
  };

  // ── Formatters ──
  const formatDate = (iso: string) => {
    try { return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" }); }
    catch { return iso; }
  };
  const formatDateTime = (iso: string) => {
    try { return new Date(iso).toLocaleString(undefined, { month: "short", day: "numeric", year: "numeric", hour: "2-digit", minute: "2-digit" }); }
    catch { return iso; }
  };

  return (
    <div className="page">
      {authWarning && (
        <div className="auth-banner">
          ⚠ acli not authenticated — run <code style={{ margin: "0 4px", background: "#3a2a0a", padding: "1px 5px", borderRadius: 3 }}>acli jira auth login</code> in your terminal
          <button className="btn-secondary" style={{ marginLeft: 12, padding: "2px 10px", fontSize: "0.8rem" }} onClick={() => void checkAuth()} disabled={checkingAuth}>
            {checkingAuth ? "Checking…" : "Check again"}
          </button>
        </div>
      )}

      <div className="topbar">
        {/* Tabs */}
        <div className="home-tabs">
          <div
            className={`home-tab${activeTab === "contexts" ? " home-tab--active" : ""}`}
            onClick={() => switchTab("contexts")}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => e.key === "Enter" && switchTab("contexts")}
          >
            <span className="home-tab-label">Contexts</span>
            {totalUnviewed > 0 && <span className="home-tab-badge">{totalUnviewed}</span>}
            <button
              className="home-tab-action"
              title="Refresh All"
              disabled={refreshingAll || reloadingAll}
              onClick={(e) => { e.stopPropagation(); void handleRefreshAll(); }}
            >
              {refreshingAll ? <Spinner size={11} color="currentColor" /> : <RefreshCw size={11} />}
            </button>
          </div>

          <div
            className={`home-tab${activeTab === "reviews" ? " home-tab--active" : ""}`}
            onClick={() => switchTab("reviews")}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => e.key === "Enter" && switchTab("reviews")}
          >
            <span className="home-tab-label">Code Reviews</span>
            {totalReviews > 0 && <span className="home-tab-badge">{totalReviews}</span>}
            <button
              className="home-tab-action"
              title="Refresh"
              disabled={refreshingReviews}
              onClick={(e) => { e.stopPropagation(); void handleRefreshReviews(); }}
            >
              {refreshingReviews ? <Spinner size={11} color="currentColor" /> : <RefreshCw size={11} />}
            </button>
          </div>

          <div
            className={`home-tab${activeTab === "repositories" ? " home-tab--active" : ""}`}
            onClick={() => switchTab("repositories")}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => e.key === "Enter" && switchTab("repositories")}
          >
            <span className="home-tab-label">Repositories</span>
            {trackedRepos.length > 0 && (
              <span className="home-tab-badge">{trackedRepos.length}</span>
            )}
          </div>
        </div>

        {/* Contextual right-side actions */}
        {activeTab === "contexts" && (
          <>
            <div className="context-search">
              <input
                type="text"
                className="context-search-input"
                placeholder="Search…"
                value={searchQuery}
                onChange={(e) => handleSearchChange(e.target.value)}
                autoCorrect="off"
                autoCapitalize="none"
                spellCheck={false}
              />
              {searchQuery && (
                <button className="context-search-clear" onClick={() => handleSearchChange("")} title="Clear search">×</button>
              )}
            </div>
            <div className="topbar-actions">
              <button
                className="btn-secondary"
                onClick={() => void handleReloadAll()}
                disabled={reloadingAll || refreshingAll}
                title="Fetch fresh data from all linked Jira queries and Slack threads"
              >
                {reloadingAll
                  ? <span className="btn-with-spinner"><Spinner size={14} color="var(--text)" /> Reloading…</span>
                  : <><RotateCcw size={16} /> Reload All</>}
              </button>
              <button className="btn-ghost topbar-settings-btn" onClick={() => openSettingsTab()} title="Settings">
                <Settings size={20} />
              </button>
              <button className="btn-primary" onClick={() => setShowForm(true)}>+ New Context</button>
            </div>
          </>
        )}

        {activeTab === "reviews" && (
          <div className="topbar-actions">
            <input
              className="context-search-input"
              style={{ width: 200, fontSize: 12 }}
              placeholder="Filter by #, title, or author…"
              value={reviewFilter}
              onChange={(e) => setReviewFilter(e.target.value)}
            />
            <button className="btn-ghost topbar-settings-btn" onClick={() => openSettingsTab()} title="Settings">
              <Settings size={20} />
            </button>
          </div>
        )}

        {activeTab === "repositories" && (
          <div className="topbar-actions">
            <button className="btn-ghost topbar-settings-btn" onClick={() => openSettingsTab()} title="Settings">
              <Settings size={20} />
            </button>
          </div>
        )}
      </div>

      <div className="content">
        {error && <div className="error-banner">{error}</div>}

        {/* ── CONTEXTS TAB ── */}
        {activeTab === "contexts" && (
          <>
            {/* Needs-attention panel — always shown when loading is done, regardless of search */}
            {!loading && needsAttentionContexts.length > 0 && (
              <div className="needs-attention-panel">
                <div className="needs-attention-header">Needs Attention</div>
                <div className="needs-attention-list">
                  {needsAttentionContexts.map((c) => {
                    return (
                      <div
                        key={c.id}
                        className="needs-attention-item"
                        role="button"
                        tabIndex={0}
                        onClick={() => openContextTab(c.id, c.key, orgs.find((o) => o.id === c.orgId)?.name, c.orgId)}
                        onKeyDown={(e) => e.key === "Enter" && openContextTab(c.id, c.key, orgs.find((o) => o.id === c.orgId)?.name, c.orgId)}
                        title={c.snapshotStatusComment || undefined}
                      >
                        <span className="needs-attention-key">{c.key}</span>
                        {c.snapshotStatusComment && (
                          <span className="needs-attention-comment">{c.snapshotStatusComment}</span>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Table area */}
            {loading ? (
              <div className="loading">Loading…</div>
            ) : contexts.length === 0 ? (
              <div className="empty-state">
                <div style={{ fontSize: 32 }}>📋</div>
                <div className="empty-state-title">No contexts yet</div>
                <div>Create a context to start saving Jira issues</div>
                <button className="btn-primary" style={{ marginTop: 8 }} onClick={() => setShowForm(true)}>Create first context</button>
              </div>
            ) : filteredContexts.length === 0 && searchQuery ? (
              <div className="empty-state">
                <div className="empty-state-title">No matches</div>
                <div>No contexts match &ldquo;{searchQuery}&rdquo;</div>
              </div>
            ) : (
              <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                <SortableContext items={filteredContexts.map((c) => c.id)} strategy={verticalListSortingStrategy}>
                  <table className="context-table">
                    <thead>
                      <tr>
                        <th style={{ width: 20 }}></th>
                        <th style={{ width: 40 }}>Active</th>
                        <th>Key</th>
                        <th style={{ width: 120 }}>Status</th>
                        <th style={{ width: 88 }}></th>
                        <th>Issues</th>
                        <th>Unread</th>
                        <th>Updated</th>
                        <th>Latest item</th>
                        <th style={{ width: 72 }}></th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredContexts.map((c) => {
                        const isActive = c.status === "active";
                        return (
                          <SortableContextRow
                            key={c.id}
                            id={c.id}
                            disabled={!isDraggingEnabled}
                            className={`context-row${isActive ? " context-row-active" : " context-row-archived"}`}
                            onClick={() => openContextTab(c.id, c.key, orgs.find((o) => o.id === c.orgId)?.name, c.orgId)}
                          >
                            {(dragHandleProps) => (
                              <>
                                <td
                                  className="drag-handle-cell"
                                  onClick={(e) => e.stopPropagation()}
                                  {...(isDraggingEnabled ? dragHandleProps : {})}
                                >
                                  {isDraggingEnabled && <GripVertical size={13} />}
                                </td>
                                <td className="active-cell" onClick={(e) => e.stopPropagation()}>
                                  <label className="active-toggle" title={isActive ? "Archive this context" : "Reactivate this context"}>
                                    <input
                                      type="checkbox"
                                      checked={isActive}
                                      onChange={() => void handleToggleStatus({ stopPropagation: () => {} } as React.MouseEvent, c)}
                                    />
                                    <span className={`active-indicator${isActive ? " active-indicator--on" : ""}`} />
                                  </label>
                                </td>
                                <td>
                                  <span className={`context-key${!isActive ? " context-key--archived" : ""}`}>{c.key}</span>
                                </td>
                                <td>
                                  <WorkflowStatusBadge
                                    status={effectiveStatus(c)}
                                    isAI={false}
                                    statuses={workflowStatuses}
                                    title={c.snapshotStatusComment || undefined}
                                  />
                                </td>
                                <td>
                                  <div className="ctx-src-icons">
                                    {c.hasJira && (
                                      <span className="ctx-src-icon ctx-src-icon--jira" title="Jira">J</span>
                                    )}
                                    {c.hasSlack && (
                                      <span className="ctx-src-icon ctx-src-icon--slack" title="Slack">
                                        <Hash size={9} strokeWidth={2.5} />
                                      </span>
                                    )}
                                    {c.hasVcs && (
                                      <span className="ctx-src-icon ctx-src-icon--vcs" title="GitHub">
                                        <GitBranch size={10} strokeWidth={1.8} />
                                      </span>
                                    )}
                                    {c.hasFiles && (
                                      <span className="ctx-src-icon ctx-src-icon--files" title="Files">
                                        <FileText size={10} strokeWidth={1.8} />
                                      </span>
                                    )}
                                  </div>
                                </td>
                                <td><span className="context-meta">{c.issueCount ?? 0}</span></td>
                                <td>
                                  {(c.unviewedCount ?? 0) > 0
                                    ? <span className="unread-badge">{c.unviewedCount}</span>
                                    : <span className="context-meta" style={{ opacity: 0.3 }}>—</span>}
                                </td>
                                <td><span className="context-meta">{formatDate(c.updatedAt)}</span></td>
                                <td>
                                  {c.lastItemAt
                                    ? <span className="context-meta context-last-item">{formatDateTime(c.lastItemAt)}</span>
                                    : <span className="context-meta" style={{ opacity: 0.35 }}>—</span>}
                                </td>
                                <td>
                                  <div className="row-actions" onClick={(e) => e.stopPropagation()}>
                                    <select
                                      className="btn-ghost"
                                      style={{ fontSize: 11 }}
                                      value={c.orgId}
                                      onChange={async (e) => {
                                        await SetContextOrg(c.id, e.target.value);
                                        void load();
                                      }}
                                      onClick={(e) => e.stopPropagation()}
                                      title="Assign to organization"
                                    >
                                      <option value="">No org</option>
                                      {orgs.map((o) => (
                                        <option key={o.id} value={o.id}>{o.name}</option>
                                      ))}
                                    </select>
                                    <button className="btn-ghost" title="Edit" onClick={(e) => { e.stopPropagation(); void openEditForm(c); }}>
                                      <Pencil size={14} />
                                    </button>
                                    <button className="btn-ghost" title="Delete" style={{ color: "var(--danger)" }} onClick={(e) => { e.stopPropagation(); setDeleteTarget(c); }}>
                                      <X size={14} />
                                    </button>
                                  </div>
                                </td>
                              </>
                            )}
                          </SortableContextRow>
                        );
                      })}
                    </tbody>
                  </table>
                </SortableContext>
              </DndContext>
            )}
          </>
        )}

        {/* ── REVIEWS TAB ── */}
        {activeTab === "reviews" && (
          <>
            {/* Repo management moved to Repositories tab */}
            <div style={{ padding: "8px 16px", fontSize: 12, color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }}>
              Repositories managed in the{" "}
              <button
                style={{ background: "none", border: "none", color: "var(--accent)", cursor: "pointer", fontSize: 12, padding: 0 }}
                onClick={() => switchTab("repositories")}
              >
                Repositories tab
              </button>
              .
            </div>

            {/* Reviews grouped by repo */}
            {repoGroups.length === 0 ? (
              <div className="detail-empty" style={{ marginTop: 32 }}>
                {trackedRepos.length === 0
                  ? "Add repositories in the Repositories tab, then click Refresh to load pending reviews."
                  : "No pending code reviews. Click Refresh to check."}
              </div>
            ) : reviewFilter.trim() && !repoGroups.some(({ reviews }) => {
                const q = reviewFilter.trim().toLowerCase();
                return reviews.some((r) => r.prNumber.toString().includes(q) || (r.prTitle ?? "").toLowerCase().includes(q) || (r.prAuthor ?? "").toLowerCase().includes(q));
              }) ? (
              <div className="detail-empty" style={{ marginTop: 32 }}>No reviews match your filter.</div>
            ) : (
              repoGroups.map(({ repo, reviews }) => {
                const url = repoUrl(repo);
                const q = reviewFilter.trim().toLowerCase();
                const match = (r: ReviewRequest) => !q || r.prNumber.toString().includes(q) || (r.prTitle ?? "").toLowerCase().includes(q) || (r.prAuthor ?? "").toLowerCase().includes(q);
                const pending = reviews.filter((r) => !r.reviewedAt && match(r));
                const waiting = reviews.filter((r) => !!r.reviewedAt && match(r));
                if (pending.length === 0 && waiting.length === 0) return null;
                return (
                  <section key={repo.id} className="detail-section review-repo-section">
                    <div className="review-repo-header">
                      <span className="platform-badge">{repo.platform}</span>
                      <span className="review-repo-name">{repo.owner}/{repo.name}</span>
                      {url && (
                        <button className="review-repo-link" onClick={() => BrowserOpenURL(url)} title={`Open on ${repo.platform}`}>
                          <ExternalLink size={12} />
                        </button>
                      )}
                      {pending.length > 0 && <span className="detail-section-count">{pending.length}</span>}
                    </div>
                    {pending.length === 0 ? (
                      <div className="detail-section-empty">No pending reviews in this repo.</div>
                    ) : (
                      <div className="pr-list">
                        {pending.map((review) => (
                          <ReviewCard
                            key={review.id}
                            review={review}
                            checkoutLoading={reviewCheckoutLoading}
                            onCheckout={() => void handleReviewCheckout(review.headBranch ?? "", review.repoId)}
                          />
                        ))}
                      </div>
                    )}
                    {waiting.length > 0 && (
                      <>
                        <div className="review-waiting-header">
                          Waiting for author
                          <span className="detail-section-count" style={{ marginLeft: 6 }}>{waiting.length}</span>
                        </div>
                        <div className="pr-list">
                          {waiting.map((review) => (
                            <ReviewCard
                              key={review.id}
                              review={review}
                              onDismiss={handleDismissReview}
                              checkoutLoading={reviewCheckoutLoading}
                              onCheckout={() => void handleReviewCheckout(review.headBranch ?? "", review.repoId)}
                            />
                          ))}
                        </div>
                      </>
                    )}
                  </section>
                );
              })
            )}

            {reviewCheckoutError && (
              <div className="checkout-error" style={{ margin: "8px 16px", fontSize: 12, color: "var(--danger)" }}>
                {reviewCheckoutError}
                <button style={{ marginLeft: 8, background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)" }} onClick={() => setReviewCheckoutError("")}>
                  <X size={11} />
                </button>
              </div>
            )}
          </>
        )}

        {/* ── REPOSITORIES TAB ── */}
        {activeTab === "repositories" && (
          <>
            {/* Add repo form */}
            <div className="repo-add-section">
              <div className="tracked-repos-bar" style={{ border: "none", padding: "16px 0 10px" }}>
                <span className="tracked-repos-label">Add Repository</span>
                <div className="tracked-repos-chips">
                  {showAddRepo ? (
                    <div className="tracked-repo-add-form">
                      <select
                        className="passphrase-input"
                        style={{ width: 110, fontSize: 11, padding: "2px 6px" }}
                        value={addPlatform}
                        onChange={(e) => setAddPlatform(e.target.value as VCSPlatform)}
                      >
                        {(platforms.length > 0 ? platforms : [{ platform: "github" as VCSPlatform, label: "GitHub", enabled: true }]).map((p) => (
                          <option key={p.platform} value={p.platform} disabled={!p.enabled}>
                            {p.label}{!p.enabled ? " (soon)" : ""}
                          </option>
                        ))}
                      </select>
                      <input
                        className="passphrase-input"
                        style={{ width: 160, fontSize: 11, padding: "2px 8px" }}
                        placeholder="owner/repo"
                        value={addOwnerRepo}
                        onChange={(e) => setAddOwnerRepo(e.target.value)}
                        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); void handleAddRepo(); } }}
                        autoFocus
                        autoCorrect="off"
                        autoCapitalize="none"
                        spellCheck={false}
                      />
                      <button
                        className="btn-primary"
                        style={{ fontSize: 11, padding: "2px 10px", whiteSpace: "nowrap" }}
                        onClick={() => void handleAddRepo()}
                        disabled={addingRepo}
                      >
                        {addingRepo ? "Adding…" : "Add"}
                      </button>
                      <button
                        className="btn-secondary"
                        style={{ fontSize: 11, padding: "2px 8px" }}
                        onClick={() => { setShowAddRepo(false); setAddRepoError(""); setAddOwnerRepo(""); }}
                      >
                        Cancel
                      </button>
                      {addRepoError && <span className="field-error">{addRepoError}</span>}
                    </div>
                  ) : (
                    <button className="tracked-repo-add-btn" onClick={() => setShowAddRepo(true)}>
                      + Add Repository
                    </button>
                  )}
                </div>
              </div>
            </div>

            {/* Repo card list */}
            {trackedRepos.length === 0 ? (
              <div className="detail-empty" style={{ marginTop: 16 }}>
                No repositories tracked yet. Click "+ Add Repository" above.
              </div>
            ) : (
              <div className="repo-list">
                {trackedRepos.map((repo) => {
                  const url = repoUrl(repo);
                  const isEditing = editingRepoId === repo.id;
                  const displayName = `${repo.owner}/${repo.name}`;
                  return (
                    <div key={repo.id} className="repo-card">
                      <div className="repo-card-header">
                        <span className="platform-badge">{repo.platform}</span>
                        <span className="repo-card-name">{displayName}</span>
                        {repo.label && <span className="repo-card-label">{repo.label}</span>}
                        <div className="repo-card-actions">
                          {url && (
                            <button onClick={() => BrowserOpenURL(url)} title={`Open on ${repo.platform}`}>
                              <ExternalLink size={12} /> Open
                            </button>
                          )}
                          <button
                            onClick={() => void handleAddRepoPath(repo.id)}
                            title="Add local path"
                          >
                            <Folder size={12} /> Add path
                          </button>
                          <button
                            onClick={() => void handleRefreshWorkspaces(repo.id)}
                            title="Refresh workspaces for all paths"
                            disabled={refreshingRepoId === repo.id}
                          >
                            <RefreshCw size={12} />
                          </button>
                          {!isEditing && (
                            <button onClick={() => startEdit(repo)} title="Edit label and description">
                              <Pencil size={12} /> Edit
                            </button>
                          )}
                          <button
                            className="danger"
                            onClick={() => void handleRemoveRepo(repo.id)}
                            title="Remove repository"
                          >
                            <X size={12} /> Remove
                          </button>
                        </div>
                      </div>

                      {(repoPaths[repo.id] ?? []).length > 0 && (
                        <div className="repo-paths-list">
                          {(repoPaths[repo.id] ?? []).map((p) => (
                            <div key={p.id}>
                              <div className="repo-path-row">
                                <FolderOpen size={11} style={{ flexShrink: 0 }} />
                                <span className="repo-path-value" title={p.path}>{p.path}</span>
                                <button
                                  className="repo-path-open"
                                  title="Open in terminal"
                                  onClick={() => void OpenRepoPathTerminal(p.id).catch(() => {})}
                                >
                                  <ExternalLink size={11} /> Open
                                </button>
                                <button
                                  className="repo-path-remove"
                                  title="Remove path"
                                  onClick={() => void handleRemoveRepoPath(p.id)}
                                >
                                  <X size={11} />
                                </button>
                              </div>
                              {(repoWorkspaces[p.id] ?? []).length > 0 && (
                                <div className="repo-section-label">Workspaces</div>
                              )}
                              {(repoWorkspaces[p.id] ?? []).map((w) => {
                                const branchUrl = url ? `${url}/tree/${encodeURIComponent(w.branch)}` : "";
                                return (
                                  <div key={w.id} className="workspace-row">
                                    <GitBranch size={10} style={{ flexShrink: 0 }} />
                                    <span className="workspace-folder">{w.worktreePath.split("/").pop()}</span>
                                    <span className="workspace-branch">{w.branch}</span>
                                    <span className={`workspace-status-badge workspace-status-badge--${w.status}`}>
                                      {w.status === "should_delete" ? "done" : "active"}
                                    </span>
                                    {branchUrl && (
                                      <button
                                        className="repo-path-open"
                                        title={`Open ${w.branch} on ${repo.platform}`}
                                        onClick={() => BrowserOpenURL(branchUrl)}
                                      >
                                        <ExternalLink size={11} /> Open
                                      </button>
                                    )}
                                    <button
                                      className="repo-path-open"
                                      title={`Open ${w.worktreePath} in terminal`}
                                      disabled={openingWorkspaceId === w.id}
                                      onClick={() => {
                                        setOpeningWorkspaceId(w.id);
                                        void OpenWorkspaceTerminal(w.id).catch(() => {}).finally(() => setOpeningWorkspaceId(null));
                                      }}
                                    >
                                      {openingWorkspaceId === w.id ? <Spinner size={11} /> : <ExternalLink size={11} />} Checkout
                                    </button>
                                    {w.status === "active" && (
                                      <button
                                        className="workspace-action-btn"
                                        title="Mark as done (ready to delete)"
                                        onClick={() => void handleMarkWorkspaceDone(w.id, repo.id)}
                                      >
                                        <Check size={10} /> Done
                                      </button>
                                    )}
                                    {w.status === "should_delete" && (
                                      <button
                                        className="workspace-action-btn workspace-action-btn--danger"
                                        title="Delete workspace"
                                        onClick={() => void handleDeleteWorkspace(w.id, repo.id)}
                                      >
                                        <Trash2 size={10} /> Delete
                                      </button>
                                    )}
                                  </div>
                                );
                              })}
                              {(localBranches[p.id] ?? []).length > 0 && (
                                <div className="repo-section-label">Local branches</div>
                              )}
                              {(localBranches[p.id] ?? []).map((b) => {
                                const branchUrl = url ? `${url}/tree/${encodeURIComponent(b.name)}` : "";
                                return (
                                  <div key={b.name} className="local-branch-row">
                                    <GitBranch size={10} style={{ flexShrink: 0, opacity: 0.5 }} />
                                    <span className="local-branch-name">{b.name}</span>
                                    {b.isCurrent && <span className="local-branch-badge local-branch-badge--current">current</span>}
                                    {b.isGone && <span className="local-branch-badge local-branch-badge--gone">gone</span>}
                                    {branchUrl && (
                                      <button
                                        className="repo-path-open"
                                        title={`Open ${b.name} on ${repo.platform}`}
                                        onClick={() => BrowserOpenURL(branchUrl)}
                                      >
                                        <ExternalLink size={11} /> Open
                                      </button>
                                    )}
                                    {!b.isCurrent && !(repoWorkspaces[p.id] ?? []).some(w => w.branch === b.name) && (
                                      <button
                                        className="repo-path-open"
                                        title={`Switch main working tree to ${b.name}`}
                                        onClick={() => void handleSwitchMainWorkingTree(p.id, b.name)}
                                      >
                                        <GitBranch size={11} /> Switch
                                      </button>
                                    )}
                                    <button
                                      className="repo-path-open"
                                      title={`Checkout ${b.name}`}
                                      onClick={() => void handleCheckoutLocalBranch(p.id, b.name, repo.id)}
                                      disabled={checkingOutKey !== null}
                                    >
                                      {checkingOutKey === `${p.id}:${b.name}`
                                        ? <Loader2 size={11} style={{ animation: "progress-spin 1s linear infinite" }} />
                                        : <ExternalLink size={11} />
                                      } Checkout
                                    </button>
                                    <button
                                      className="repo-path-open repo-path-open--danger"
                                      title={`Delete local branch ${b.name}`}
                                      onClick={() => void handleDeleteLocalBranch(p.id, b.name)}
                                    >
                                      <X size={11} /> Remove
                                    </button>
                                  </div>
                                );
                              })}
                            </div>
                          ))}
                        </div>
                      )}

                      {repo.description && !isEditing && (
                        <div className="repo-card-description">{repo.description}</div>
                      )}

                      {isEditing && (
                        <div className="repo-card-edit-form">
                          <div className="repo-card-edit-row">
                            <label>Label</label>
                            <input
                              value={editLabel}
                              onChange={(e) => setEditLabel(e.target.value)}
                              placeholder="e.g. API, Frontend, Infra"
                              autoFocus
                              autoCorrect="off"
                              autoCapitalize="none"
                              spellCheck={false}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") { e.preventDefault(); void handleUpdateRepoMeta(repo.id); }
                                if (e.key === "Escape") cancelEdit();
                              }}
                            />
                          </div>
                          <div className="repo-card-edit-row">
                            <label>Description</label>
                            <input
                              value={editDescription}
                              onChange={(e) => setEditDescription(e.target.value)}
                              placeholder="Short description (optional)"
                              autoCorrect="off"
                              autoCapitalize="none"
                              spellCheck={false}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") { e.preventDefault(); void handleUpdateRepoMeta(repo.id); }
                                if (e.key === "Escape") cancelEdit();
                              }}
                            />
                          </div>
                          <div className="repo-card-edit-row">
                            <label>On checkout</label>
                            <input
                              value={editOnCheckoutCommand}
                              onChange={(e) => setEditOnCheckoutCommand(e.target.value)}
                              placeholder="Command to run after checkout (e.g. code .)"
                              autoCorrect="off"
                              autoCapitalize="none"
                              spellCheck={false}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") { e.preventDefault(); void handleUpdateRepoMeta(repo.id); }
                                if (e.key === "Escape") cancelEdit();
                              }}
                            />
                          </div>
                          {editError && <span className="field-error">{editError}</span>}
                          <div style={{ display: "flex", gap: 6 }}>
                            <button
                              className="btn-primary"
                              style={{ fontSize: 11, padding: "2px 10px" }}
                              onClick={() => void handleUpdateRepoMeta(repo.id)}
                              disabled={editSaving}
                            >
                              {editSaving ? "Saving…" : "Save"}
                            </button>
                            <button
                              className="btn-secondary"
                              style={{ fontSize: 11, padding: "2px 8px" }}
                              onClick={cancelEdit}
                            >
                              Cancel
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </>
        )}
      </div>

      {/* Modals */}
      {showForm && <ContextForm onSave={handleCreate} onCancel={() => setShowForm(false)} />}

      {editTarget && (
        <ContextForm
          initial={editTarget}
          initialJqlQuery={editJqlQuery}
          initialJiraIssues={editJiraIssues}
          initialSlackChannels={editSlackChannels}
          initialSlackDMConfigs={editSlackDMConfigs}
          initialContextTexts={editContextTexts}
          initialUploadedFiles={editUploadedFiles}
          initialRelations={editRelations}
          onSave={handleUpdate}
          onCancel={closeEditForm}
        />
      )}

      {deleteTarget && (
        <div className="modal-overlay">
          <div className="modal">
            <h2>Delete Context</h2>
            <p className="confirm-msg">
              Are you sure you want to delete <strong>{deleteTarget.key}</strong>?<br />
              This will remove all saved issues and comments.
            </p>
            <div className="modal-footer">
              <button className="btn-secondary" onClick={() => setDeleteTarget(null)}>Cancel</button>
              <button className="btn-danger" onClick={handleDelete} disabled={deleting}>{deleting ? "Deleting…" : "Delete"}</button>
            </div>
          </div>
        </div>
      )}

      {reloadResults !== null && (
        <div className="modal-overlay">
          <div className="modal reload-results-modal">
            <h2>Reload Complete</h2>
            {reloadResults.length === 0 ? (
              <p className="confirm-msg">No contexts to reload.</p>
            ) : (
              <div className="reload-results-list">
                {reloadResults.map((r) => {
                  const hasNew = r.newIssues > 0 || r.newComments > 0 || r.threadsRefreshed > 0 || r.dmsRefreshed > 0 || r.prsRefreshed > 0 || r.newPRComments > 0 || r.newNotifications > 0 || r.newJiraStatusChanges > 0;
                  return (
                    <div
                      key={r.contextId}
                      className={`reload-result-item${!r.error ? " reload-result-item--clickable" : ""}${hasNew ? " reload-result-item--new" : ""}`}
                      {...(!r.error && {
                        role: "button" as const,
                        tabIndex: 0,
                        onClick: () => { const ctxOrgId = contexts.find((c) => c.id === r.contextId)?.orgId; const orgName = orgs.find((o) => o.id === ctxOrgId)?.name; openNewContextTab(r.contextId, r.contextKey, orgName, ctxOrgId); setReloadResults(null); },
                        onKeyDown: (e: React.KeyboardEvent) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); const ctxOrgId = contexts.find((c) => c.id === r.contextId)?.orgId; const orgName = orgs.find((o) => o.id === ctxOrgId)?.name; openNewContextTab(r.contextId, r.contextKey, orgName, ctxOrgId); setReloadResults(null); } },
                      })}
                    >
                      <span className="reload-result-key">{r.contextKey}</span>
                      <span className="reload-result-summary">
                        {r.error ? (
                          <span className="reload-result-error">{r.error}</span>
                        ) : hasNew ? (
                          <>
                            {r.newIssues > 0 && <span className="reload-result-badge">{r.newIssues} new issue{r.newIssues !== 1 ? "s" : ""}</span>}
                            {r.newComments > 0 && <span className="reload-result-badge">{r.newComments} new comment{r.newComments !== 1 ? "s" : ""}</span>}
                            {r.threadsRefreshed > 0 && <span className="reload-result-badge">{r.threadsRefreshed} thread{r.threadsRefreshed !== 1 ? "s" : ""} refreshed</span>}
                            {r.dmsRefreshed > 0 && <span className="reload-result-badge">{r.dmsRefreshed} DM{r.dmsRefreshed !== 1 ? "s" : ""} refreshed</span>}
                            {r.prsRefreshed > 0 && <span className="reload-result-badge">{r.prsRefreshed} PR{r.prsRefreshed !== 1 ? "s" : ""} synced</span>}
                            {r.newPRComments > 0 && <span className="reload-result-badge">{r.newPRComments} PR comment{r.newPRComments !== 1 ? "s" : ""}</span>}
                            {r.newNotifications > 0 && <span className="reload-result-badge">{r.newNotifications} notification{r.newNotifications !== 1 ? "s" : ""}</span>}
                            {r.newJiraStatusChanges > 0 && <span className="reload-result-badge">{r.newJiraStatusChanges} Jira status change{r.newJiraStatusChanges !== 1 ? "s" : ""}</span>}
                          </>
                        ) : (
                          <span className="reload-result-none">No new items</span>
                        )}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
            <div className="modal-footer">
              <button className="btn-primary" onClick={() => setReloadResults(null)}>Done</button>
            </div>
          </div>
        </div>
      )}
      {reviewCheckoutTarget && reviewCheckoutPaths.length > 1 && (
        <div className="modal-overlay">
          <div className="modal">
            <h2>Select Local Path</h2>
            <p className="confirm-msg">
              Choose which local clone to use for checking out <strong>{reviewCheckoutTarget.branch}</strong>:
            </p>
            <div style={{ display: "flex", flexDirection: "column", gap: 8, margin: "12px 0" }}>
              {reviewCheckoutLoading ? (
                <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 0", fontSize: 12, color: "var(--text-muted)" }}>
                  <Spinner size={14} /> Checking out…
                </div>
              ) : reviewCheckoutPaths.map((p) => (
                <button
                  key={p.id}
                  className="btn-secondary"
                  style={{ textAlign: "left", padding: "8px 12px", fontSize: 12 }}
                  onClick={() => void handleReviewCheckoutWithPath(p.id)}
                >
                  {p.path}
                </button>
              ))}
            </div>
            {reviewCheckoutError && <div className="error-banner">{reviewCheckoutError}</div>}
            <div className="modal-footer">
              <button className="btn-secondary" onClick={dismissReviewCheckoutModal}>
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
      <CheckoutLogModal
        open={reviewLogOpen}
        running={reviewLogRunning}
        logs={reviewLogs}
        command={reviewLogCommand}
        exitCode={reviewLogExitCode}
        onClose={dismissReviewLog}
      />
    </div>
  );
}
