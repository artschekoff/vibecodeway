import { useState, useEffect, useCallback, useRef } from "react";
import Spinner from "../components/Spinner";
import { useNavigate, useParams } from "react-router-dom";
import {
  GetContextDetail,
  RefreshContext,
  CancelRefresh,
  DeleteJiraIssue,
  DeleteJiraComment,
  DeleteSlackThread,
  RemoveSlackChannel,
  DeleteContextFile,
  ToggleIssueActive,
  ToggleCommentActive,
  ToggleSlackChannelActive,
  ToggleSlackThreadActive,
  ToggleContextFileActive,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { service } from "../../wailsjs/go/models";
import IssueCard from "../components/IssueCard";
import SlackChannelCard from "../components/SlackChannelCard";
import { useProgress } from "../contexts/ProgressContext";

export default function ContextDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const progress = useProgress();
  const [detail, setDetail] = useState<service.ContextDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [syncMsg, setSyncMsg] = useState("");
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const d = await GetContextDetail(id);
      setDetail(d);
    } catch {
      // Wails bridge not available or load failed — fail silently
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const handleRefresh = async () => {
    if (!id) return;
    setRefreshing(true);
    setError("");
    setSyncMsg("");

    progress.begin("Refreshing Context", [], () => { void CancelRefresh(); });

    const unsubscribe = EventsOn(
      "refresh:progress",
      (data: { current: number; total: number; step: string }) => {
        const label = data.step === "jira" ? "Syncing Jira issues & comments"
          : data.step === "slack" ? "Refreshing Slack threads"
          : `Syncing ${data.step}`;
        progress.set(data.current - 1, data.total, label);
      }
    );

    try {
      const result = await RefreshContext(id);
      const parts: string[] = [];
      if (result.issuesRefreshed > 0) parts.push(`${result.issuesRefreshed} issue${result.issuesRefreshed !== 1 ? "s" : ""} updated`);
      if (result.newComments > 0) parts.push(`${result.newComments} new comment${result.newComments !== 1 ? "s" : ""}`);
      if (result.threadsRefreshed > 0) parts.push(`${result.threadsRefreshed} Slack thread${result.threadsRefreshed !== 1 ? "s" : ""} refreshed`);
      const msg = parts.length > 0 ? parts.join(" · ") : "Everything is up to date.";
      setSyncMsg(msg);
      if (toastTimer.current) clearTimeout(toastTimer.current);
      toastTimer.current = setTimeout(() => setSyncMsg(""), 4000);
      load();
    } catch (e) {
      setError(String(e));
    } finally {
      unsubscribe();
      EventsOff("refresh:progress");
      setRefreshing(false);
      progress.done();
    }
  };

  const showToast = (msg: string) => {
    setSyncMsg(msg);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setSyncMsg(""), 3000);
  };

  const handleDeleteIssue = async (issueId: string) => {
    try {
      await DeleteJiraIssue(issueId);
      showToast("Issue removed");
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDeleteComment = async (commentId: string) => {
    try {
      await DeleteJiraComment(commentId);
      showToast("Comment removed");
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDeleteChannel = async (channelId: string) => {
    try {
      await RemoveSlackChannel(channelId);
      showToast("Channel removed");
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDeleteThread = async (threadId: string) => {
    try {
      await DeleteSlackThread(threadId);
      showToast("Thread removed");
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleDeleteContextFile = async (fileId: string) => {
    try {
      await DeleteContextFile(fileId);
      showToast("Text file removed");
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleToggleIssue = async (issueId: string) => {
    try {
      await ToggleIssueActive(issueId);
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleToggleComment = async (commentId: string) => {
    try {
      await ToggleCommentActive(commentId);
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleToggleChannel = async (channelId: string) => {
    try {
      await ToggleSlackChannelActive(channelId);
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleToggleThread = async (threadId: string) => {
    try {
      await ToggleSlackThreadActive(threadId);
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  const handleToggleContextFile = async (fileId: string) => {
    try {
      await ToggleContextFileActive(fileId);
      load();
    } catch (e) {
      setError(String(e));
    }
  };

  if (loading) return <div className="loading">Loading…</div>;

  if (!detail) return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => navigate("/")}>← Back</button>
      </div>
      <div className="content">
        <div className="error-banner">{error || "Context not found"}</div>
      </div>
    </div>
  );

  const { context } = detail;
  const issues = detail.issues ?? [];
  const slackChannels = detail.slackChannels ?? [];
  const slackThreads = detail.slackThreads ?? [];
  const contextFiles = detail.contextFiles ?? [];
  const isEmpty = issues.length === 0 && slackChannels.length === 0 && slackThreads.length === 0 && contextFiles.length === 0;

  return (
    <div className="page">
      <div className="topbar">
        <button className="back-btn" onClick={() => navigate("/")}>← Back</button>
        <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 10 }}>
          <div>
            <h1 style={{ fontSize: 16, margin: 0 }}>{context.key}</h1>
            <div style={{ display: "flex", gap: 12, marginTop: 2 }}>
              {issues.length > 0 && (
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {issues.length} issue{issues.length !== 1 ? "s" : ""}
                </span>
              )}
              {slackChannels.length > 0 && (
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {slackChannels.length} Slack channel{slackChannels.length !== 1 ? "s" : ""}
                </span>
              )}
              {slackThreads.length > 0 && (
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {slackThreads.length} thread{slackThreads.length !== 1 ? "s" : ""}
                </span>
              )}
              {contextFiles.length > 0 && (
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {contextFiles.length} text file{contextFiles.length !== 1 ? "s" : ""}
                </span>
              )}
            </div>
          </div>
          <span className={`badge badge-${context.status}`}>{context.status}</span>
        </div>
        <div className="topbar-actions">
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
              <><span style={{ fontSize: 18, lineHeight: 1 }}>⟳</span> Refresh</>
            )}
          </button>
        </div>
      </div>

      <div className="content">
        {error && <div className="error-banner">{error}</div>}
        {syncMsg && <div className="toast toast--success">{syncMsg}</div>}

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
                  <span className="detail-section-title">Jira</span>
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
                    />
                  ))}
                </div>
              </section>
            )}

            {/* Slack section */}
            {(slackChannels.length > 0 || slackThreads.length > 0) && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">Slack</span>
                  <span className="detail-section-count">
                    {slackChannels.length + slackThreads.length}
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
                      />
                    ))}
                </div>
              </section>
            )}
            {/* Text files section */}
            {contextFiles.length > 0 && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">Texts</span>
                  <span className="detail-section-count">{contextFiles.length}</span>
                </div>
                <div className="issue-list">
                  {contextFiles.map((f) => (
                    <div key={f.id} className={`text-file-card${!f.isActive ? " text-file-card--inactive" : ""}`}>
                      <div className="text-file-card-header">
                        <label
                          className="item-active-toggle"
                          title={f.isActive ? "Active — click to deactivate" : "Inactive — click to activate"}
                          onClick={(e) => { e.stopPropagation(); void handleToggleContextFile(f.id); }}
                        >
                          <input type="checkbox" checked={f.isActive} onChange={() => {}} />
                          <span className={`item-active-indicator${f.isActive ? " item-active-indicator--on" : ""}`} />
                        </label>
                        <span className={`text-file-card-name${!f.isActive ? " item--inactive" : ""}`}>{f.name || "Untitled"}</span>
                        <button
                          type="button"
                          className="btn-ghost"
                          style={{ color: "var(--danger)", fontSize: 12 }}
                          title="Remove text file"
                          onClick={() => void handleDeleteContextFile(f.id)}
                        >✕</button>
                      </div>
                      <pre className="text-file-card-content">{f.content}</pre>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
