import { useState, useEffect, useCallback } from "react";
import Spinner from "../components/Spinner";
import { useNavigate } from "react-router-dom";
import {
  ListAllContexts,
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
  UpsertContextText,
  DeleteContextText,
  DeleteUploadedFile,
  ToggleContextStatus,
  UpsertSlackDMConfig,
  DeleteSlackDMConfig,
  ListSlackDMConfigs,
  CommitPendingUploads,
} from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import type { ContextText, Context, Issue, SlackChannel, SlackDMConfig, SlackSavePayload, TextFilesPayload, UploadedFile } from "../types";
import type { service } from "../../wailsjs/go/models";
import ContextForm from "../components/ContextForm";
import { useProgress } from "../contexts/ProgressContext";

export default function ContextList() {
  const navigate = useNavigate();
  const progress = useProgress();
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
  const [deleteTarget, setDeleteTarget] = useState<Context | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [reloadingAll, setReloadingAll] = useState(false);
  const [reloadResults, setReloadResults] = useState<service.ReloadAllResult[] | null>(null);

  const load = useCallback(async () => {
    try {
      const list = await ListAllContexts();
      setContexts(list ?? []);
    } catch {
      // Wails bridge not available (browser dev mode) or load failed — fail silently
    } finally {
      setLoading(false);
    }
  }, []);

  const checkAuth = useCallback(async () => {
    setCheckingAuth(true);
    try {
      const s = await AuthStatus();
      setAuthWarning(s.toLowerCase().includes("not logged") || s.toLowerCase().includes("error"));
    } catch {
      // Wails bridge not available (browser dev mode) — ignore
    } finally {
      setCheckingAuth(false);
    }
  }, []);

  useEffect(() => {
    load();
    void checkAuth();
  }, [load, checkAuth]);

  const handleCreate = async (
    key: string,
    searchQuery: string,
    _status: string | undefined,
    slackPayload?: SlackSavePayload,
    selectedJiraKeys?: string[],
    _removedJiraIssueIds?: string[],
    textFilesPayload?: TextFilesPayload,
    _removedUploadedFileIds?: string[],
    pendingUploadTempIds?: string[]
  ) => {
    const jiraKeys = selectedJiraKeys ?? [];
    const slackGroups = (slackPayload?.groups ?? []).filter((g) => g.channels.length > 0);
    const dmConfigs = slackPayload?.dmConfigs ?? [];
    const newTextFiles = textFilesPayload?.newFiles ?? [];
    const pendingIds = pendingUploadTempIds ?? [];
    const itemLabels = [
      ...jiraKeys,
      ...slackGroups.flatMap((g) =>
        g.channels.map((ch) => `#${ch.channelName}  ·  ${g.workspace}`)
      ),
      ...dmConfigs.map((dm) => `DMs from ${dm.workspace}`),
      ...newTextFiles.map((f) => f.name || "Text file"),
      ...pendingIds.map(() => "Uploading file"),
    ];

    progress.begin("Creating Context", itemLabels);

    try {
      const ctx = await CreateContext(key);
      if (searchQuery.trim()) {
        await SetJiraConfig(ctx.id, searchQuery.trim());
      }

      for (const jiraKey of jiraKeys) {
        progress.next(`Adding issue ${jiraKey}`);
        try { await AddIssuesToContext(ctx.id, [jiraKey]); } catch { /* best-effort */ }
      }

      for (const group of slackGroups) {
        const names = group.channels.map((ch) => `#${ch.channelName}`).join(", ");
        progress.next(`Adding ${names} from ${group.workspace}`);
        try { await AddSlackChannels(ctx.id, group.workspace, group.channels); } catch { /* best-effort */ }
      }

      for (const dm of dmConfigs) {
        progress.next(`Loading DMs from ${dm.workspace}`);
        try { await UpsertSlackDMConfig(ctx.id, dm.workspace, dm.startFrom); } catch { /* best-effort */ }
      }

      for (const f of newTextFiles) {
        progress.next(`Saving ${f.name || "text file"}`);
        try { await UpsertContextText("", ctx.id, f.name, f.content); } catch { /* best-effort */ }
      }

      if (pendingIds.length > 0) {
        progress.next("Saving uploaded files");
        try { await CommitPendingUploads(ctx.id, pendingIds); } catch { /* best-effort */ }
      }
    } finally {
      progress.done();
    }

    setShowForm(false);
    load();
  };

  const openEditForm = async (c: Context) => {
    let jql = "";
    let issues: Issue[] = [];
    let channels: SlackChannel[] = [];
    let dmConfigs: SlackDMConfig[] = [];
    let contextTexts: ContextText[] = [];
    let uploadedFiles: UploadedFile[] = [];
    try {
      const [detail, dms] = await Promise.all([
        GetContextDetail(c.id),
        ListSlackDMConfigs(c.id),
      ]);
      jql = detail.jira?.searchQuery ?? "";
      issues = detail.issues ?? [];
      channels = detail.slackChannels ?? [];
      contextTexts = detail.contextTexts ?? [];
      uploadedFiles = (detail.uploadedFiles ?? []) as UploadedFile[];
      dmConfigs = dms ?? [];
    } catch {
      // best-effort — open with empty values
    }
    setEditJqlQuery(jql);
    setEditJiraIssues(issues);
    setEditSlackChannels(channels);
    setEditSlackDMConfigs(dmConfigs);
    setEditContextTexts(contextTexts);
    setEditUploadedFiles(uploadedFiles);
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
  };

  const handleUpdate = async (
    key: string,
    searchQuery: string,
    status: string | undefined,
    slackPayload?: SlackSavePayload,
    selectedJiraKeys?: string[],
    _removedJiraIssueIds?: string[],
    textFilesPayload?: TextFilesPayload,
    removedUploadedFileIds?: string[]
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
      ...slackGroups.flatMap((g) =>
        g.channels.map((ch) => `#${ch.channelName}  ·  ${g.workspace}`)
      ),
      ...dmConfigs.map((dm) => `DMs from ${dm.workspace}`),
      ...newTextFiles.map((f) => f.name || "Text file"),
    ];

    progress.begin("Updating Context", itemLabels);

    try {
      await UpdateContext(editTarget.id, key, status ?? editTarget.status);
      if (searchQuery.trim()) {
        await SetJiraConfig(editTarget.id, searchQuery.trim());
      }

      for (const jiraKey of jiraKeys) {
        progress.next(`Adding issue ${jiraKey}`);
        try { await AddIssuesToContext(editTarget.id, [jiraKey]); } catch { /* best-effort */ }
      }

      for (const id of removedChannelIds) {
        try { await RemoveSlackChannel(id); } catch { /* best-effort */ }
      }

      for (const group of slackGroups) {
        const names = group.channels.map((ch) => `#${ch.channelName}`).join(", ");
        progress.next(`Adding ${names} from ${group.workspace}`);
        try { await AddSlackChannels(editTarget.id, group.workspace, group.channels); } catch { /* best-effort */ }
      }

      for (const id of removedDMConfigIds) {
        try { await DeleteSlackDMConfig(id); } catch { /* best-effort */ }
      }

      for (const dm of dmConfigs) {
        progress.next(`Loading DMs from ${dm.workspace}`);
        try { await UpsertSlackDMConfig(editTarget.id, dm.workspace, dm.startFrom); } catch { /* best-effort */ }
      }

      for (const id of removedTextFileIds) {
        try { await DeleteContextText(id); } catch { /* best-effort */ }
      }

      for (const f of editedTextFiles) {
        try { await UpsertContextText(f.id, editTarget.id, f.name, f.content); } catch { /* best-effort */ }
      }

      for (const f of newTextFiles) {
        progress.next(`Saving ${f.name || "text file"}`);
        try { await UpsertContextText("", editTarget.id, f.name, f.content); } catch { /* best-effort */ }
      }

      for (const fId of (removedUploadedFileIds ?? [])) {
        try { await DeleteUploadedFile(fId); } catch { /* best-effort */ }
      }
    } finally {
      progress.done();
    }

    closeEditForm();
    load();
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await DeleteContext(deleteTarget.id);
      setDeleteTarget(null);
      load();
    } finally {
      setDeleting(false);
    }
  };

  const handleReloadAll = async () => {
    setReloadingAll(true);

    // Start with empty items list — items are discovered as events arrive
    progress.begin("Reloading All Contexts", []);

    const unsubscribe = EventsOn(
      "reload:progress",
      (data: { current: number; total: number; contextKey: string }) => {
        // 0-based index = current - 1
        progress.set(data.current - 1, data.total, `Processing ${data.contextKey}`);
      }
    );

    try {
      const results = await ReloadAllContexts();
      setReloadResults(results ?? []);
      load();
    } catch {
      setError("Reload failed — check logs for details.");
    } finally {
      unsubscribe();
      EventsOff("reload:progress");
      setReloadingAll(false);
      progress.done();
    }
  };

  const handleToggleStatus = async (e: React.MouseEvent, c: Context) => {
    e.stopPropagation();
    try {
      const updated = await ToggleContextStatus(c.id);
      setContexts((prev) =>
        prev.map((ctx) => (ctx.id === updated.id ? { ...ctx, status: updated.status } : ctx))
      );
    } catch {
      // best-effort
    }
  };

  const formatDate = (iso: string) => {
    try {
      return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
    } catch {
      return iso;
    }
  };

  const formatDateTime = (iso: string) => {
    try {
      return new Date(iso).toLocaleString(undefined, {
        month: "short", day: "numeric", year: "numeric",
        hour: "2-digit", minute: "2-digit",
      });
    } catch {
      return iso;
    }
  };

  return (
    <div className="page">
      {authWarning && (
        <div className="auth-banner">
          ⚠ acli not authenticated — run <code style={{ margin: "0 4px", background: "#3a2a0a", padding: "1px 5px", borderRadius: 3 }}>acli jira auth login</code> in your terminal
          <button
            className="btn-secondary"
            style={{ marginLeft: 12, padding: "2px 10px", fontSize: "0.8rem" }}
            onClick={() => void checkAuth()}
            disabled={checkingAuth}
          >
            {checkingAuth ? "Checking…" : "Check again"}
          </button>
        </div>
      )}
      <div className="topbar">
        <h1>Contexts</h1>
        <div className="topbar-actions">
          <button
            className="btn-secondary"
            onClick={() => void handleReloadAll()}
            disabled={reloadingAll}
            title="Fetch fresh data from all linked Jira queries and Slack threads"
          >
            {reloadingAll ? (
              <span className="btn-with-spinner">
                <Spinner size={14} color="var(--text)" />
                Reloading…
              </span>
            ) : "↺ Reload All"}
          </button>
          <button
            className="btn-ghost"
            onClick={() => navigate("/settings")}
            title="OpenAI settings"
          >
            ⚙
          </button>
          <button className="btn-primary" onClick={() => setShowForm(true)}>+ New Context</button>
        </div>
      </div>

      <div className="content">
        {error && <div className="error-banner">{error}</div>}

        {loading ? (
          <div className="loading">Loading…</div>
        ) : contexts.length === 0 ? (
          <div className="empty-state">
            <div style={{ fontSize: 32 }}>📋</div>
            <div className="empty-state-title">No contexts yet</div>
            <div>Create a context to start saving Jira issues</div>
            <button className="btn-primary" style={{ marginTop: 8 }} onClick={() => setShowForm(true)}>Create first context</button>
          </div>
        ) : (
          <table className="context-table">
            <thead>
              <tr>
                <th style={{ width: 40 }}>Active</th>
                <th>Key</th>
                <th>Issues</th>
                <th>Updated</th>
                <th>Latest item</th>
                <th style={{ width: 72 }}></th>
              </tr>
            </thead>
            <tbody>
              {contexts.map((c) => {
                const isActive = c.status === "active";
                return (
                  <tr
                    key={c.id}
                    className={`context-row${isActive ? " context-row-active" : " context-row-archived"}`}
                    onClick={() => navigate(`/context/${c.id}`)}
                  >
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
                    <td><span className={`context-key${!isActive ? " context-key--archived" : ""}`}>{c.key}</span></td>
                    <td><span className="context-meta">{c.issueCount ?? 0}</span></td>
                    <td><span className="context-meta">{formatDate(c.updatedAt)}</span></td>
                    <td>
                      {c.lastItemAt ? (
                        <span className="context-meta context-last-item">{formatDateTime(c.lastItemAt)}</span>
                      ) : (
                        <span className="context-meta" style={{ opacity: 0.35 }}>—</span>
                      )}
                    </td>
                    <td>
                      <div className="row-actions" onClick={(e) => e.stopPropagation()}>
                        <button
                          className="btn-ghost"
                          title="Edit"
                          onClick={(e) => { e.stopPropagation(); void openEditForm(c); }}
                        >✏</button>
                        <button
                          className="btn-ghost"
                          title="Delete"
                          style={{ color: "var(--danger)" }}
                          onClick={(e) => { e.stopPropagation(); setDeleteTarget(c); }}
                        >✕</button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {showForm && (
        <ContextForm
          onSave={handleCreate}
          onCancel={() => setShowForm(false)}
        />
      )}

      {editTarget && (
        <ContextForm
          initial={editTarget}
          initialJqlQuery={editJqlQuery}
          initialJiraIssues={editJiraIssues}
          initialSlackChannels={editSlackChannels}
          initialSlackDMConfigs={editSlackDMConfigs}
          initialContextTexts={editContextTexts}
          initialUploadedFiles={editUploadedFiles}
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
              <button className="btn-danger" onClick={handleDelete} disabled={deleting}>
                {deleting ? "Deleting…" : "Delete"}
              </button>
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
                  const hasNew = r.newIssues > 0 || r.newComments > 0 || r.threadsRefreshed > 0 || r.dmsRefreshed > 0;
                  return (
                    <div key={r.contextId} className={`reload-result-item${hasNew ? " reload-result-item--new" : ""}`}>
                      <span className="reload-result-key">{r.contextKey}</span>
                      <span className="reload-result-summary">
                        {r.error ? (
                          <span className="reload-result-error">{r.error}</span>
                        ) : hasNew ? (
                          [
                            r.newIssues > 0 && `${r.newIssues} new issue${r.newIssues !== 1 ? "s" : ""}`,
                            r.newComments > 0 && `${r.newComments} new comment${r.newComments !== 1 ? "s" : ""}`,
                            r.threadsRefreshed > 0 && `${r.threadsRefreshed} thread${r.threadsRefreshed !== 1 ? "s" : ""} refreshed`,
                            r.dmsRefreshed > 0 && `${r.dmsRefreshed} DM${r.dmsRefreshed !== 1 ? "s" : ""} refreshed`,
                          ].filter(Boolean).join(" · ")
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
    </div>
  );
}
