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
  ToggleSlackDMConfigActive,
  UpdateContext,
  SetJiraConfig,
  AddIssuesToContext,
  AddSlackChannels,
  UpsertContextFile,
  UpsertSlackDMConfig,
  DeleteSlackDMConfig,
  RemoveSlackDMEntry,
  ListSlackDMConfigs,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { service } from "../../wailsjs/go/models";
import type { Context, Issue, SlackChannel, ContextFile, SlackDMConfig, SlackSavePayload, TextFilesPayload } from "../types";
import IssueCard from "../components/IssueCard";
import SlackChannelCard from "../components/SlackChannelCard";
import SlackDMConfigCard from "../components/SlackDMConfigCard";
import ContextChatPanel from "../components/ContextChatPanel";
import ContextLogList from "../components/ContextLogList";
import ConsolidatedLogView from "../components/ConsolidatedLogView";
import ContextForm from "../components/ContextForm";
import { useProgress } from "../contexts/ProgressContext";

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
  const [editContextFiles, setEditContextFiles] = useState<ContextFile[] | undefined>(undefined);

  const load = useCallback(async (silent = false) => {
    if (!id) return;
    if (!silent) setLoading(true);
    try {
      const d = await GetContextDetail(id);
      setDetail(d);
    } catch {
      // Wails bridge not available or load failed — fail silently
    } finally {
      if (!silent) setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const handleRefresh = async () => {
    if (!id) return;
    setRefreshing(true);
    setRefreshError("");
    setToastMsg("");

    progress.begin("Refreshing Context", [], () => { void CancelRefresh(); });

    const unsubscribe = EventsOn(
      "refresh:progress",
      (data: { current: number; total: number; step: string }) => {
        const label = data.step === "jira" ? "Syncing Jira issues & comments"
          : data.step === "slack" ? "Refreshing Slack threads & DMs"
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
      if (result.dmsRefreshed > 0) parts.push(`${result.dmsRefreshed} DM${result.dmsRefreshed !== 1 ? "s" : ""} refreshed`);
      const msg = parts.length > 0 ? parts.join(" · ") : "Everything is up to date.";
      showToast(msg);
      load(true);
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
    let contextFiles: ContextFile[] = [];
    try {
      const [det, dms] = await Promise.all([
        GetContextDetail(id),
        ListSlackDMConfigs(id),
      ]);
      jql = det.jira?.searchQuery ?? "";
      issues = det.issues ?? [];
      channels = det.slackChannels ?? [];
      contextFiles = det.contextFiles ?? [];
      dmConfigs = dms ?? [];
    } catch {
      // best-effort — open with empty values
    }
    setEditJqlQuery(jql);
    setEditJiraIssues(issues);
    setEditSlackChannels(channels);
    setEditSlackDMConfigs(dmConfigs);
    setEditContextFiles(contextFiles);
    setEditOpen(true);
  };

  const closeEditForm = () => {
    setEditOpen(false);
    setEditJqlQuery(undefined);
    setEditJiraIssues(undefined);
    setEditSlackChannels(undefined);
    setEditSlackDMConfigs(undefined);
    setEditContextFiles(undefined);
  };

  const handleUpdate = async (
    key: string,
    searchQuery: string,
    status: string | undefined,
    slackPayload?: SlackSavePayload,
    selectedJiraKeys?: string[],
    _removedJiraIssueIds?: string[],
    textFilesPayload?: TextFilesPayload
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
        try { await DeleteContextFile(fId); } catch { /* best-effort */ }
      }
      for (const f of editedTextFiles) {
        try { await UpsertContextFile(f.id, id, f.name, f.content); } catch { /* best-effort */ }
      }
      for (const f of newTextFiles) {
        progress.next(`Saving ${f.name || "text file"}`);
        try { await UpsertContextFile("", id, f.name, f.content); } catch { /* best-effort */ }
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

  const handleDeleteContextFile = async (fileId: string) => {
    try {
      await DeleteContextFile(fileId);
      showToast("Text file removed");
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

  const handleToggleContextFile = async (fileId: string) => {
    try {
      await ToggleContextFileActive(fileId);
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
        <button className="back-btn" onClick={() => navigate("/")}>← Back</button>
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
  const contextFiles = detail.contextFiles ?? [];
  const isEmpty = issues.length === 0 && slackChannels.length === 0 && slackThreads.length === 0 && slackDMConfigs.length === 0 && contextFiles.length === 0;

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
              {slackDMConfigs.length > 0 && (
                <span style={{ fontSize: 11, color: "var(--text-muted)" }}>
                  {slackDMConfigs.length} DM config{slackDMConfigs.length !== 1 ? "s" : ""}
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
            className="btn-secondary"
            onClick={() => void openEditForm()}
            title="Edit context"
          >
            ✏ Edit
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
              <><span style={{ fontSize: 18, lineHeight: 1 }}>⟳</span> Refresh</>
            )}
          </button>
        </div>
      </div>

      <div className="content content--split">
        {refreshError && <div className="error-banner">{refreshError}</div>}
        {toastMsg && <div className={`toast toast--${toastType}`}>{toastMsg}</div>}

        <div className="detail-primary">
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
            {(slackChannels.length > 0 || slackThreads.length > 0 || slackDMConfigs.length > 0) && (
              <section className="detail-section">
                <div className="detail-section-header">
                  <span className="detail-section-title">Slack</span>
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

                  {/* DM configs */}
                  {slackDMConfigs.map((dm) => (
                    <SlackDMConfigCard
                      key={dm.id}
                      dm={dm}
                      onDelete={(id) => void handleDeleteDMConfig(id)}
                      onToggle={(id) => void handleToggleDMConfig(id)}
                      onDeleteEntry={(dmConfigId, entryId) => void handleDeleteDMEntry(dmConfigId, entryId)}
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
          initialContextFiles={editContextFiles}
          onSave={handleUpdate}
          onCancel={closeEditForm}
        />
      )}
    </div>
  );
}
