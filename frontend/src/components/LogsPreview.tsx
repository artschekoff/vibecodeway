import { useEffect, useState } from "react";
import { ListAllLogs, GetAppLogs } from "../../wailsjs/go/main/App";
import type { LogEntry, AppLogEntry } from "../types";

const TRIGGERS = [
  { value: "", label: "All" },
  { value: "refresh", label: "Refresh" },
  { value: "reload", label: "Reload" },
  { value: "manual", label: "Manual" },
  { value: "init", label: "Init" },
  { value: "agent", label: "Agent" },
];

const LIMIT = 100;

function formatDate(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

function formatTs(ts: number): string {
  try {
    return new Date(ts * 1000).toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return String(ts);
  }
}

function ContextLogsTab() {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [trigger, setTrigger] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    ListAllLogs(trigger, LIMIT)
      .then((data) => setEntries(data ?? []))
      .finally(() => setLoading(false));
  }, [trigger]);

  return (
    <>
      <div className="settings-logs-filter-bar">
        <label htmlFor="logs-level-filter">Level</label>
        <select
          id="logs-level-filter"
          value={trigger}
          onChange={(e) => setTrigger(e.target.value)}
        >
          {TRIGGERS.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </div>

      {loading ? (
        <div className="settings-logs-empty">Loading…</div>
      ) : entries.length === 0 ? (
        <div className="settings-logs-empty">No log entries found.</div>
      ) : (
        <table className="settings-logs-table">
          <thead>
            <tr>
              <th className="settings-logs-cell-date">Date</th>
              <th className="settings-logs-cell-level">Level</th>
              <th className="settings-logs-cell-context">Context</th>
              <th className="settings-logs-cell-org">Organization</th>
              <th className="settings-logs-cell-message">Message</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e) => (
              <tr key={e.id}>
                <td className="settings-logs-cell-date">
                  {formatDate(e.createdAt)}
                </td>
                <td className="settings-logs-cell-level">
                  <span
                    className={`settings-logs-trigger settings-logs-trigger--${e.trigger}`}
                  >
                    {e.trigger}
                  </span>
                </td>
                <td className="settings-logs-cell-context">{e.contextKey}</td>
                <td className="settings-logs-cell-org">{e.orgName ?? "—"}</td>
                <td className="settings-logs-cell-message" title={e.summaryMd}>
                  {e.summaryMd}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

function AppErrorsTab() {
  const [entries, setEntries] = useState<AppLogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    GetAppLogs(LIMIT)
      .then((data) => setEntries(data ?? []))
      .finally(() => setLoading(false));
  }, []);

  return (
    <>
      {loading ? (
        <div className="settings-logs-empty">Loading…</div>
      ) : entries.length === 0 ? (
        <div className="settings-logs-empty">No app errors found.</div>
      ) : (
        <table className="settings-logs-table">
          <thead>
            <tr>
              <th className="settings-logs-cell-date">Date</th>
              <th className="settings-logs-cell-level">Level</th>
              <th className="settings-logs-cell-context">Caller</th>
              <th className="settings-logs-cell-message">Message</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e, i) => (
              <tr key={i}>
                <td className="settings-logs-cell-date">{formatTs(e.ts)}</td>
                <td className="settings-logs-cell-level">
                  <span className={`settings-logs-trigger settings-logs-trigger--${e.level}`}>
                    {e.level}
                  </span>
                </td>
                <td className="settings-logs-cell-context">{e.caller}</td>
                <td
                  className="settings-logs-cell-message"
                  title={e.error ? `${e.msg}\n${e.error}` : e.msg}
                >
                  {e.msg}
                  {e.error && (
                    <span className="app-log-error-detail"> — {e.error}</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

type Tab = "context" | "app";

export default function LogsPreview() {
  const [tab, setTab] = useState<Tab>("app");

  return (
    <div className="settings-logs-section">
      <div className="detail-section-header">Logs</div>

      <div className="settings-logs-tabs">
        <button
          className={`settings-logs-tab${tab === "app" ? " settings-logs-tab--active" : ""}`}
          onClick={() => setTab("app")}
        >
          App Errors
        </button>
        <button
          className={`settings-logs-tab${tab === "context" ? " settings-logs-tab--active" : ""}`}
          onClick={() => setTab("context")}
        >
          Context Logs
        </button>
      </div>

      {tab === "context" ? <ContextLogsTab /> : <AppErrorsTab />}
    </div>
  );
}
