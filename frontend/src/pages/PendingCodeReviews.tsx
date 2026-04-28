import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeft, Check, Copy, ExternalLink, Folder, FolderOpen, RefreshCw, X } from "lucide-react";
import {
  GetPendingCodeReviews,
  RefreshPendingReviews,
  ListTrackedRepos,
  AddTrackedRepo,
  RemoveTrackedRepo,
  GetSupportedPlatforms,
  DismissReviewRequest,
  PickRepoDirectory,
  SetTrackedRepoPath,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { VCSRepo, ReviewRequest, VCSPlatform, VCSPlatformInfo } from "../types";
import Spinner from "../components/Spinner";

function repoUrl(repo: VCSRepo): string {
  if (repo.platform === "github") return `https://github.com/${repo.owner}/${repo.name}`;
  if (repo.platform === "gitlab") return `https://gitlab.com/${repo.owner}/${repo.name}`;
  return "";
}

function ReviewCard({ review, onDismiss }: { review: ReviewRequest; onDismiss?: (id: string) => void }) {
  const [copied, setCopied] = useState(false);
  const isWaiting = !!review.reviewedAt;

  const handleCopy = () => {
    if (!review.prUrl) return;
    void navigator.clipboard.writeText(review.prUrl).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  };

  return (
    <div className={`review-card${isWaiting ? " review-card--waiting" : ""}`}>
      <div className="review-card-top">
        <span className="review-card-number">#{review.prNumber}</span>
        <span className="review-card-title">{review.prTitle || "(no title)"}</span>
        {isWaiting && (
          <span className="pr-status-badge pr-status--waiting">Waiting for Author</span>
        )}
      </div>
      <div className="review-card-bottom">
        <span className="review-card-author">by {review.prAuthor || "unknown"}</span>
        <div className="review-card-btns">
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

interface RepoGroup {
  repo: VCSRepo;
  reviews: ReviewRequest[];
}

export default function PendingCodeReviews() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [repoGroups, setRepoGroups] = useState<RepoGroup[]>([]);
  const [trackedRepos, setTrackedRepos] = useState<VCSRepo[]>([]);
  const [platforms, setPlatforms] = useState<VCSPlatformInfo[]>([]);
  const [showAddRepo, setShowAddRepo] = useState(false);
  const [addPlatform, setAddPlatform] = useState<VCSPlatform>("github");
  const [addOwnerRepo, setAddOwnerRepo] = useState("");
  const [addError, setAddError] = useState("");
  const [adding, setAdding] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [reviews, repos, plats] = await Promise.all([
        GetPendingCodeReviews() as Promise<ReviewRequest[]>,
        ListTrackedRepos("github") as Promise<VCSRepo[]>,
        GetSupportedPlatforms() as Promise<VCSPlatformInfo[]>,
      ]);

      setTrackedRepos(repos ?? []);
      setPlatforms(plats ?? []);

      // Group reviews by repo
      const groups: RepoGroup[] = (repos ?? []).map((repo) => ({
        repo,
        reviews: (reviews ?? []).filter((r) => r.repoId === repo.id),
      }));
      setRepoGroups(groups);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const handleRefresh = async () => {
    setRefreshing(true);
    setError("");
    try {
      await RefreshPendingReviews();
      await loadData();
    } catch (e) {
      setError(String(e));
    } finally {
      setRefreshing(false);
    }
  };

  const handleDismiss = async (id: string) => {
    try {
      await DismissReviewRequest(id);
      await loadData();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleAddRepo = async () => {
    const trimmed = addOwnerRepo.trim();
    if (!trimmed || !trimmed.includes("/")) {
      setAddError("Enter owner/repo (e.g. octocat/hello-world)");
      return;
    }
    const [owner, ...rest] = trimmed.split("/");
    const name = rest.join("/");
    if (!owner || !name) {
      setAddError("Enter owner/repo (e.g. octocat/hello-world)");
      return;
    }
    setAdding(true);
    setAddError("");
    try {
      await AddTrackedRepo(addPlatform, `${owner}/${name}`);
      setAddOwnerRepo("");
      setShowAddRepo(false);
      await loadData();
    } catch (e) {
      setAddError(String(e));
    } finally {
      setAdding(false);
    }
  };

  const handleRemoveRepo = async (repoId: string) => {
    try {
      await RemoveTrackedRepo(repoId);
      await loadData();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleSetRepoPath = async (repoId: string) => {
    try {
      const path = await PickRepoDirectory();
      if (!path) return;
      await SetTrackedRepoPath(repoId, path);
      await loadData();
    } catch (e) { setError(String(e)); }
  };

  if (loading) return <div className="loading"><Spinner /> Loading…</div>;

  const totalReviews = repoGroups.reduce((n, g) => n + g.reviews.filter((r) => !r.reviewedAt).length, 0);

  return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => navigate("/")}><ArrowLeft size={16} /> Back</button>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>
            Pending Code Reviews
            {totalReviews > 0 && (
              <span className="detail-section-count" style={{ marginLeft: 8 }}>{totalReviews}</span>
            )}
          </h1>
        </div>
        <button
          className="btn-secondary"
          onClick={() => void handleRefresh()}
          disabled={refreshing}
        >
          {refreshing ? <Spinner /> : <><RefreshCw size={14} /> Refresh</>}
        </button>
      </div>

      <div className="content">
        {error && <div className="error-banner">{error}</div>}

        {/* Tracked Repositories */}
        <div className="tracked-repos-bar">
          <span className="tracked-repos-label">Tracked repositories</span>
          <div className="tracked-repos-chips">
            {trackedRepos.map((repo) => {
              const url = repoUrl(repo);
              return (
                <div key={repo.id} className="tracked-repo-chip">
                  <span className="platform-badge">{repo.platform}</span>
                  <span className="tracked-repo-name">{repo.owner}/{repo.name}</span>
                  <button
                    className={`tracked-repo-link${repo.localPath ? " tracked-repo-link--active" : ""}`}
                    onClick={() => void handleSetRepoPath(repo.id)}
                    title={repo.localPath ? `Local path: ${repo.localPath}` : "Set local path"}
                  >
                    {repo.localPath ? <FolderOpen size={11} /> : <Folder size={11} />}
                  </button>
                  {url && (
                    <button
                      className="tracked-repo-link"
                      onClick={() => BrowserOpenURL(url)}
                      title={`Open ${repo.owner}/${repo.name} on ${repo.platform}`}
                    >
                      <ExternalLink size={11} />
                    </button>
                  )}
                  <button
                    className="tracked-repo-remove"
                    onClick={() => void handleRemoveRepo(repo.id)}
                    title="Remove"
                  >
                    <X size={11} />
                  </button>
                </div>
              );
            })}
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
                />
                <button className="btn-primary" style={{ fontSize: 11, padding: "2px 10px", whiteSpace: "nowrap" }} onClick={() => void handleAddRepo()} disabled={adding}>
                  {adding ? "Adding…" : "Add"}
                </button>
                <button className="btn-secondary" style={{ fontSize: 11, padding: "2px 8px" }} onClick={() => { setShowAddRepo(false); setAddError(""); setAddOwnerRepo(""); }}>
                  Cancel
                </button>
                {addError && <span className="field-error">{addError}</span>}
              </div>
            ) : (
              <button
                className="tracked-repo-add-btn"
                onClick={() => setShowAddRepo(true)}
              >
                + Add
              </button>
            )}
          </div>
        </div>

        {/* Reviews grouped by repo */}
        {repoGroups.length === 0 ? (
          <div className="detail-empty" style={{ marginTop: 32 }}>
            {trackedRepos.length === 0
              ? "Add repositories above, then click Refresh to load pending reviews."
              : "No pending code reviews. Click Refresh to check."}
          </div>
        ) : (
          repoGroups.map(({ repo, reviews }) => {
            const url = repoUrl(repo);
            const pending = reviews.filter((r) => !r.reviewedAt);
            const waiting = reviews.filter((r) => !!r.reviewedAt);
            if (pending.length === 0 && waiting.length === 0) return null;
            return (
              <section key={repo.id} className="detail-section review-repo-section">
                <div className="review-repo-header">
                  <span className="platform-badge">{repo.platform}</span>
                  <span className="review-repo-name">{repo.owner}/{repo.name}</span>
                  {url && (
                    <button
                      className="review-repo-link"
                      onClick={() => BrowserOpenURL(url)}
                      title={`Open on ${repo.platform}`}
                    >
                      <ExternalLink size={12} />
                    </button>
                  )}
                  {pending.length > 0 && (
                    <span className="detail-section-count">{pending.length}</span>
                  )}
                </div>

                {pending.length === 0 ? (
                  <div className="detail-section-empty">No pending reviews in this repo.</div>
                ) : (
                  <div className="pr-list">
                    {pending.map((review) => (
                      <ReviewCard key={review.id} review={review} />
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
                        <ReviewCard key={review.id} review={review} onDismiss={handleDismiss} />
                      ))}
                    </div>
                  </>
                )}
              </section>
            );
          })
        )}
      </div>
    </div>
  );
}
