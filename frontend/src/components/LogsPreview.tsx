import { useEffect, useState } from "react";
import { ListAllLogs } from "../../wailsjs/go/main/App";
import type { LogEntry } from "../types";

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

export default function LogsPreview() {
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
    <div className="settings-logs-section">
      <div className="detail-section-header">Logs</div>

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
    </div>
  );
}
