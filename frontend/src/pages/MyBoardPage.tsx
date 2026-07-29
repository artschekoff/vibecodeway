import { useState, useEffect, useCallback } from "react";
import { ExternalLink, RefreshCw, AlertTriangle, GitPullRequest, ClipboardList, Eye } from "lucide-react";
import { GetMyBoard, GetVCSAuth, GetPendingCodeReviews, DismissReviewRequest, RefreshMyAssignedJiraIssues } from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { useTabContext } from "../contexts/TabContext";
import type { store, vcsstore } from "../../wailsjs/go/models";

const PERSON_NAME_KEY = "cc:board-person-name";

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
  const { openContextTab } = useTabContext();

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

  const refreshJira = async () => {
    setRefreshingJira(true);
    setJiraError("");
    try {
      // acli's workitem search only returns priority (not created), so sort is priority-only.
      setJiraIssues((await RefreshMyAssignedJiraIssues("priority")) ?? []);
    } catch (e) {
      setJiraError(e instanceof Error ? e.message : "Failed to fetch assigned issues");
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
            onClick={() => void refreshJira()} disabled={refreshingJira}>
            {refreshingJira ? "Refreshing…" : "Refresh"}
          </button>
          <span style={{ fontSize: 11, color: "var(--text-muted)" }}>by priority</span>
        </div>
        {jiraError && (
          <div className="board-section-empty" style={{ color: "var(--danger, #e5534b)" }}>{jiraError}</div>
        )}
        {jiraIssues.length === 0 ? (
          <div className="board-section-empty">No assigned Jira issues. Hit Refresh to fetch them.</div>
        ) : (
          <div className="board-section-items">
            {jiraIssues.map((it) => (
              <div key={it.key} className="board-item" style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <span style={{ fontWeight: 600 }}>{it.key}</span>
                <span style={{ flex: 1 }}>{it.summary}</span>
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{it.status}</span>
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>{it.priority}</span>
                {!it.seen && <span className="board-badge-new" style={{ fontSize: 10, color: "var(--accent)" }}>new</span>}
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
