import { useState, useEffect, useCallback, useMemo } from "react";
import { ExternalLink, RefreshCw, AlertTriangle, Activity, GitPullRequest, ClipboardList, Eye, Copy, Check, FolderPlus, FolderOpen, MessageSquare } from "lucide-react";
import { GetMyBoard, GetVCSAuth, GetPendingCodeReviews, DismissReviewRequest, RefreshMyAssignedJiraIssues, ReconcileMyOpenPRs, CreateContextFromJiraIssue, ListOrganizations, GetJiraInstanceURL } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { useTabContext } from "../contexts/TabContext";
import type { store, vcsstore } from "../../wailsjs/go/models";

const PERSON_NAME_KEY = "cc:board-person-name";
const PRIORITY_RANK: Record<string, number> = { Highest: 5, High: 4, Medium: 3, Low: 2, Lowest: 1 };
type JiraSort = "priority" | "status" | "date";

// ponytail: minute/hour/day granularity is enough for a feed; no i18n lib.
function timeAgo(iso: string): string {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export default function MyBoardPage() {
  const [personName, setPersonName] = useState(() => localStorage.getItem(PERSON_NAME_KEY) ?? "");
  const [nameInput, setNameInput] = useState(personName);
  const [data, setData] = useState<store.MyBoardData | null>(null);
  const [reviews, setReviews] = useState<vcsstore.ReviewRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [jiraIssues, setJiraIssues] = useState<store.BoardJiraAssignment[]>([]);
  const [refreshingJira, setRefreshingJira] = useState(false);
  const [jiraError, setJiraError] = useState("");
  const [orgs, setOrgs] = useState<store.Organization[]>([]);
  const [selectedOrgId, setSelectedOrgId] = useState("");
  const [jiraBaseUrl, setJiraBaseUrl] = useState("");
  const [creatingKey, setCreatingKey] = useState("");
  const [copiedKey, setCopiedKey] = useState("");
  const [jiraSort, setJiraSort] = useState<JiraSort>("priority");
  const [statusFilter, setStatusFilter] = useState("");
  const [priorityFilter, setPriorityFilter] = useState("");
  const { openContextTab } = useTabContext();

  // Distinct filter options present in the current issues.
  const statusOptions = useMemo(() => [...new Set(jiraIssues.map((i) => i.status).filter(Boolean))].sort(), [jiraIssues]);
  const priorityOptions = useMemo(() => {
    const present = new Set(jiraIssues.map((i) => i.priority).filter(Boolean));
    return ["Highest", "High", "Medium", "Low", "Lowest"].filter((p) => present.has(p));
  }, [jiraIssues]);

  // Client-side sort of the assigned issues (instant, no re-fetch). Note: acli's
  // search can't return the Jira created date, so "date" falls back to fetchedAt.
  const sortedJira = useMemo(() => {
    const arr = [...jiraIssues];
    if (jiraSort === "status") {
      arr.sort((a, b) => (a.status || "").localeCompare(b.status || ""));
    } else if (jiraSort === "date") {
      const d = (x: store.BoardJiraAssignment) => x.created || x.fetchedAt || "";
      arr.sort((a, b) => d(b).localeCompare(d(a)));
    } else {
      arr.sort((a, b) => (PRIORITY_RANK[b.priority] ?? 0) - (PRIORITY_RANK[a.priority] ?? 0));
    }
    return arr;
  }, [jiraIssues, jiraSort]);

  const visibleJira = useMemo(
    () => sortedJira.filter((i) => (!statusFilter || i.status === statusFilter) && (!priorityFilter || i.priority === priorityFilter)),
    [sortedJira, statusFilter, priorityFilter],
  );

  const load = useCallback(async (name: string) => {
    setLoading(true);
    setError(null);
    try {
      const [board, pending] = await Promise.all([
        GetMyBoard(name) as Promise<store.MyBoardData>,
        GetPendingCodeReviews() as Promise<vcsstore.ReviewRequest[]>,
      ]);
      setData(board);
      setJiraIssues((board.myAssignedJiraIssues as store.BoardJiraAssignment[]) ?? []);
      setReviews(pending ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  // On mount: prefill from GitHub auth if no name saved
  useEffect(() => {
    const init = async () => {
      let name = localStorage.getItem(PERSON_NAME_KEY) ?? "";
      if (!name) {
        try {
          const auth = await GetVCSAuth("github") as vcsstore.VCSAuth;
          if (auth?.username) {
            name = auth.username;
            setPersonName(name);
            setNameInput(name);
            localStorage.setItem(PERSON_NAME_KEY, name);
          }
        } catch { /* no auth configured */ }
      }
      void load(name);
    };
    void init();
  }, [load]);

  // Load orgs (for "Create context") + the Jira base URL (for per-ticket links) once.
  useEffect(() => {
    void ListOrganizations().then((rows) => {
      const list = (rows as store.Organization[]) ?? [];
      setOrgs(list);
      setSelectedOrgId((prev) => prev || list[0]?.id || "");
    }).catch(() => { /* no orgs */ });
    void GetJiraInstanceURL().then((u) => setJiraBaseUrl(u ?? "")).catch(() => { /* no jira */ });
  }, []);

  const jiraUrl = (key: string) => (jiraBaseUrl ? `${jiraBaseUrl}/browse/${key}` : "");

  const copyJiraLink = (key: string) => {
    const u = jiraUrl(key);
    if (!u) return;
    void navigator.clipboard?.writeText(u);
    setCopiedKey(key);
    window.setTimeout(() => setCopiedKey((k) => (k === key ? "" : k)), 1200);
  };

  const createContextForIssue = async (key: string) => {
    if (!selectedOrgId) { setJiraError("Pick an organization to create a context."); return; }
    setCreatingKey(key);
    setJiraError("");
    try {
      await CreateContextFromJiraIssue(selectedOrgId, key);
      await load(personName); // re-read the board so the row shows the new context link
    } catch (e) {
      setJiraError(e instanceof Error ? e.message : "Failed to create context");
    } finally {
      setCreatingKey("");
    }
  };

  const handleNameSave = () => {
    const trimmed = nameInput.trim();
    localStorage.setItem(PERSON_NAME_KEY, trimmed);
    setPersonName(trimmed);
    void load(trimmed);
  };

  const handleDismissReview = async (id: string) => {
    await DismissReviewRequest(id);
    setReviews((prev) => prev.filter((r) => r.id !== id));
  };

  // Board refresh: re-fetch assigned Jira issues AND reconcile PR states (a merged
  // PR otherwise lingers on "My Open PRs" as state='open'), then re-read the board so
  // myOpenPRs reflects the reconciled states.
  const refreshBoard = async () => {
    setRefreshingJira(true);
    setJiraError("");
    try {
      // acli's workitem search only returns priority (not created), so sort is priority-only.
      const [issues] = await Promise.all([
        RefreshMyAssignedJiraIssues("priority"),
        ReconcileMyOpenPRs(),
      ]);
      setJiraIssues(issues ?? []);
      const board = (await GetMyBoard(personName)) as store.MyBoardData;
      setData(board);
    } catch (e) {
      setJiraError(e instanceof Error ? e.message : "Failed to refresh");
    } finally {
      setRefreshingJira(false);
    }
  };

  return (
    <div className="board-page">
      {/* Header */}
      <div className="board-header">
        <span className="board-title">My Board</span>
        <div className="board-name-row">
          <input
            className="board-name-input"
            value={nameInput}
            onChange={(e) => setNameInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && handleNameSave()}
            placeholder="Your name / GitHub username"
            title="Used to filter assignments and PRs"
          />
          <button className="board-name-save" onClick={handleNameSave}>Apply</button>
        </div>
        <button className="board-refresh-btn" onClick={() => load(personName)} disabled={loading}>
          <RefreshCw size={14} className={loading ? "spinning" : ""} />
        </button>
      </div>

      {error && <div className="board-error">{error}</div>}

      {/* Section 0: Recent Activity */}
      <BoardSection
        icon={<Activity size={14} />}
        title="Recent Activity"
        count={data?.recentActivity?.length ?? 0}
        empty="No recent activity."
      >
        {data?.recentActivity?.map((e) => (
          <div key={e.eventId} className="board-card">
            <div className="board-card-context"
              onClick={() => openContextTab(e.contextId, e.contextKey)}
              title="Open context"
            >
              {e.contextKey}
            </div>
            <div className="board-card-body">{e.summary || e.type}</div>
            <div className="board-card-meta">{timeAgo(e.createdAt)}</div>
          </div>
        ))}
      </BoardSection>

      {/* Section 1: My Blockers */}
      <BoardSection
        icon={<AlertTriangle size={14} />}
        title="My Blockers"
        count={data?.blockers?.length ?? 0}
        empty="No recent blockers."
      >
        {data?.blockers?.map((b) => (
          <div key={b.eventId} className="board-card">
            <div className="board-card-context"
              onClick={() => openContextTab(b.contextId, b.contextKey)}
              title="Open context"
            >
              {b.contextKey}
            </div>
            <div className="board-card-body">{b.summary || "(no summary)"}</div>
            {b.author && <div className="board-card-meta">by {b.author}</div>}
          </div>
        ))}
      </BoardSection>

      {/* Section 2: My Active Tasks */}
      <BoardSection
        icon={<ClipboardList size={14} />}
        title="My Active Tasks"
        count={data?.myAssignments?.length ?? 0}
        empty="No assignments found. Check your name above matches your assignment name."
      >
        {data?.myAssignments?.map((a) => (
          <div key={a.id} className="board-card">
            <div className="board-card-top">
              <span
                className="board-card-context"
                onClick={() => openContextTab(a.contextId, a.contextKey)}
                title="Open context"
              >
                {a.contextKey}
              </span>
              {a.isOutdated && <span className="board-badge board-badge--outdated">outdated</span>}
            </div>
            <div className="board-card-body board-card-body--preview">
              {firstLine(a.body)}
            </div>
          </div>
        ))}
      </BoardSection>

      {/* My Jira To-Do */}
      <div>
        <div style={{ display: "flex", gap: 8, alignItems: "center", margin: "8px 0" }}>
          <span className="board-section-title">My Jira To-Do</span>
          <button className="btn-secondary" style={{ fontSize: 11 }} aria-label="refresh assigned jira"
            onClick={() => void refreshBoard()} disabled={refreshingJira}>
            {refreshingJira ? "Refreshing…" : "Refresh"}
          </button>
          <select aria-label="sort assigned jira" style={{ fontSize: 11 }} value={jiraSort}
            onChange={(e) => setJiraSort(e.target.value as JiraSort)} title="Sort assigned issues">
            <option value="priority">Sort: Priority</option>
            <option value="status">Sort: Status</option>
            <option value="date">Sort: Date</option>
          </select>
          {statusOptions.length > 0 && (
            <select aria-label="filter by status" style={{ fontSize: 11 }} value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)} title="Filter by status">
              <option value="">All statuses</option>
              {statusOptions.map((s) => <option key={s} value={s}>{s}</option>)}
            </select>
          )}
          {priorityOptions.length > 0 && (
            <select aria-label="filter by priority" style={{ fontSize: 11 }} value={priorityFilter}
              onChange={(e) => setPriorityFilter(e.target.value)} title="Filter by priority">
              <option value="">All priorities</option>
              {priorityOptions.map((p) => <option key={p} value={p}>{p}</option>)}
            </select>
          )}
          {orgs.length > 1 && (
            <select aria-label="organization for new contexts" style={{ fontSize: 11, marginLeft: "auto" }}
              value={selectedOrgId} onChange={(e) => setSelectedOrgId(e.target.value)}
              title="Organization used when creating a context from a ticket">
              {orgs.map((o) => <option key={o.id} value={o.id}>{o.name}</option>)}
            </select>
          )}
        </div>
        {jiraError && (
          <div className="board-section-empty" style={{ color: "var(--danger, #e5534b)" }}>{jiraError}</div>
        )}
        {jiraIssues.length === 0 ? (
          <div className="board-section-empty">No assigned Jira issues. Hit Refresh to fetch them.</div>
        ) : visibleJira.length === 0 ? (
          <div className="board-section-empty">No issues match the filter.</div>
        ) : (
          <div className="board-section-items">
            {visibleJira.map((it) => (
              <div key={it.key} className="board-item" style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <span style={{ fontWeight: 600, flexShrink: 0 }}>{it.key}</span>
                <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={it.summary}>{it.summary}</span>
                <span style={{ display: "flex", alignItems: "center", gap: 6, flexShrink: 0 }}>
                  {it.status && <span className="board-chip" title="Status">{it.status}</span>}
                  {it.priority && <span className="board-chip" title="Priority">{it.priority}</span>}
                  {!it.seen && <span className="board-badge-new" style={{ fontSize: 10, color: "var(--accent)" }}>new</span>}
                  {jiraBaseUrl && (
                    <button className="board-icon-btn" aria-label={`open ${it.key} in Jira`} title="Open in Jira"
                      onClick={() => BrowserOpenURL(jiraUrl(it.key))}><ExternalLink size={12} /></button>
                  )}
                  {jiraBaseUrl && (
                    <button className="board-icon-btn" aria-label={`copy ${it.key} link`} title={copiedKey === it.key ? "Copied" : "Copy Jira link"}
                      onClick={() => copyJiraLink(it.key)}>{copiedKey === it.key ? <Check size={12} /> : <Copy size={12} />}</button>
                  )}
                  {it.contextId ? (
                    <button className="board-icon-btn" style={{ color: "var(--accent)" }} aria-label={`open context ${it.contextKey}`} title={`Open context ${it.contextKey}`}
                      onClick={() => openContextTab(it.contextId!, it.contextKey!)}><FolderOpen size={12} /></button>
                  ) : (
                    <button className="board-icon-btn" aria-label={`create context from ${it.key}`}
                      title="Create a context from this ticket" disabled={creatingKey === it.key || (orgs.length > 0 && !selectedOrgId)}
                      onClick={() => void createContextForIssue(it.key)}>{creatingKey === it.key ? "…" : <FolderPlus size={12} />}</button>
                  )}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Section 3: My Open PRs */}
      <BoardSection
        icon={<GitPullRequest size={14} />}
        title="My Open PRs"
        count={data?.myOpenPRs?.length ?? 0}
        empty="No open PRs."
      >
        {data?.myOpenPRs?.map((pr) => (
          <div key={pr.id} className="board-item" style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span className="board-card-context" style={{ flexShrink: 0 }}
              onClick={() => openContextTab(pr.contextId, pr.contextKey)} title={pr.repoName}>
              {pr.contextKey}
            </span>
            <span className="board-pr-number">#{pr.prNumber}</span>
            <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12 }} title={pr.title}>{pr.title}</span>
            {pr.unresolvedCount > 0 && (
              <span className="board-badge" title={`${pr.unresolvedCount} unresolved comment(s)`} style={{ display: "inline-flex", alignItems: "center", gap: 2 }}>
                <MessageSquare size={11} /> {pr.unresolvedCount}
              </span>
            )}
            {pr.isDraft && <span className="board-badge board-badge--draft">draft</span>}
            {pr.ciStatus === "FAILURE" && <span className="board-badge board-badge--fail">CI</span>}
            {pr.reviewStatus === "changes_requested" && <span className="board-badge board-badge--changes">changes</span>}
            {pr.reviewStatus === "approved" && pr.ciStatus === "SUCCESS" && pr.mergeable === "MERGEABLE" && (
              <span className="board-badge board-badge--ready">ready</span>
            )}
            {pr.url && (
              <button className="board-open-btn" style={{ flexShrink: 0 }} title="Open PR" onClick={() => BrowserOpenURL(pr.url)}>
                <ExternalLink size={11} />
              </button>
            )}
          </div>
        ))}
      </BoardSection>

      {/* Section 4: PR Reviews */}
      <BoardSection
        icon={<Eye size={14} />}
        title="PR Reviews"
        count={reviews.filter((r) => !r.reviewedAt && !r.dismissedAt).length}
        empty="No pending reviews."
      >
        {reviews.filter((r) => !r.reviewedAt && !r.dismissedAt).map((r) => (
          <div key={r.id} className="board-item" style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span className="board-pr-number">#{r.prNumber}</span>
            <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: 12 }} title={r.prTitle}>{r.prTitle}</span>
            <span className="board-card-meta" style={{ flexShrink: 0 }}>by {r.prAuthor}</span>
            {r.prUrl && (
              <button className="board-open-btn" style={{ flexShrink: 0 }} title="Open PR" onClick={() => BrowserOpenURL(r.prUrl)}>
                <ExternalLink size={11} />
              </button>
            )}
            <button className="board-dismiss-btn" style={{ flexShrink: 0 }} onClick={() => handleDismissReview(r.id)}>Dismiss</button>
          </div>
        ))}
      </BoardSection>
    </div>
  );
}

function BoardSection({
  icon, title, count, empty, children,
}: {
  icon: React.ReactNode;
  title: string;
  count: number;
  empty: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="board-section">
      <div className="board-section-header">
        {icon}
        <span className="board-section-title">{title}</span>
        {count > 0 && <span className="board-section-count">{count}</span>}
      </div>
      {count === 0 ? (
        <div className="board-section-empty">{empty}</div>
      ) : (
        <div className="board-section-items">{children}</div>
      )}
    </div>
  );
}

function firstLine(body: string): string {
  const first = (body ?? "").split("\n").find((l) => l.trim().length > 0) ?? "";
  return first.replace(/^-\s+\[[ x]\]\s+/, "").slice(0, 120);
}
