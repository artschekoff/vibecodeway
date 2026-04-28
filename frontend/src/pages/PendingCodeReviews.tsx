import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeft, RefreshCw, X, ExternalLink } from "lucide-react";
import {
  GetPendingCodeReviews,
  RefreshPendingReviews,
  ListTrackedRepos,
  AddTrackedRepo,
  RemoveTrackedRepo,
  GetSupportedPlatforms,
} from "../../wailsjs/go/main/App";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { VCSRepo, ReviewRequest, VCSPlatform, VCSPlatformInfo } from "../types";
import Spinner from "../components/Spinner";

function repoUrl(repo: VCSRepo): string {
  if (repo.platform === "github") return `https://github.com/${repo.owner}/${repo.name}`;
  if (repo.platform === "gitlab") return `https://gitlab.com/${repo.owner}/${repo.name}`;
  return "";
}

function ReviewCard({ review }: { review: ReviewRequest }) {
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
    <div className="review-card">
      <div className="review-card-top">
        <span className="review-card-number">#{review.prNumber}</span>
        <span className="review-card-title">{review.prTitle || "(no title)"}</span>
        {statusLabel && (
          <span className={`pr-status-badge${statusClass}`}>{statusLabel}</span>
        )}
      </div>
      <div className="review-card-bottom">
        <span className="review-card-author">by {review.prAuthor || "unknown"}</span>
        {review.prUrl && (
          <button
            className="review-open-btn"
            onClick={() => BrowserOpenURL(review.prUrl)}
            title="Open PR"
          >
            Open <ExternalLink size={11} />
          </button>
        )}
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

  if (loading) return <div className="loading"><Spinner /> Loading…</div>;

  const totalReviews = repoGroups.reduce((n, g) => n + g.reviews.length, 0);

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
                  {reviews.length > 0 && (
                    <span className="detail-section-count">{reviews.length}</span>
                  )}
                </div>
                {reviews.length === 0 ? (
                  <div className="detail-section-empty">No pending reviews in this repo.</div>
                ) : (
                  <div className="pr-list">
                    {reviews.map((review) => (
                      <ReviewCard key={review.id} review={review} />
                    ))}
                  </div>
                )}
              </section>
            );
          })
        )}
      </div>
    </div>
  );
}
