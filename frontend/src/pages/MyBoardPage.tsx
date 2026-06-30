import { useState, useEffect, useCallback } from "react";
import { ExternalLink, RefreshCw, AlertTriangle, GitPullRequest, ClipboardList, Eye } from "lucide-react";
import { GetMyBoard, GetVCSAuth, GetPendingCodeReviews, DismissReviewRequest } from "../../wailsjs/go/main/App";
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

      {/* Section 3: My Open PRs */}
      <BoardSection
        icon={<GitPullRequest size={14} />}
        title="My Open PRs"
        count={data?.myOpenPRs?.length ?? 0}
        empty="No open PRs."
      >
        {data?.myOpenPRs?.map((pr) => (
          <div key={pr.id} className="board-card">
            <div className="board-card-top">
              <span
                className="board-card-context"
                onClick={() => openContextTab(pr.contextId, pr.contextKey)}
              >
                {pr.contextKey}
              </span>
              <span className="board-card-meta">{pr.repoName}</span>
              {pr.isDraft && <span className="board-badge board-badge--draft">draft</span>}
              {pr.ciStatus === "FAILURE" && <span className="board-badge board-badge--fail">CI failed</span>}
              {pr.reviewStatus === "changes_requested" && <span className="board-badge board-badge--changes">changes requested</span>}
              {pr.reviewStatus === "approved" && pr.ciStatus === "SUCCESS" && pr.mergeable === "MERGEABLE" && (
                <span className="board-badge board-badge--ready">ready to merge</span>
              )}
            </div>
            <div className="board-card-body">
              <span className="board-pr-number">#{pr.prNumber}</span> {pr.title}
            </div>
            {pr.url && (
              <button className="board-open-btn" onClick={() => BrowserOpenURL(pr.url)}>
                Open <ExternalLink size={11} />
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
          <div key={r.id} className="board-card">
            <div className="board-card-body">
              <span className="board-pr-number">#{r.prNumber}</span> {r.prTitle}
            </div>
            <div className="board-card-meta">by {r.prAuthor}</div>
            <div className="board-card-actions">
              {r.prUrl && (
                <button className="board-open-btn" onClick={() => BrowserOpenURL(r.prUrl)}>
                  Open <ExternalLink size={11} />
                </button>
              )}
              <button className="board-dismiss-btn" onClick={() => handleDismissReview(r.id)}>
                Dismiss
              </button>
            </div>
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
