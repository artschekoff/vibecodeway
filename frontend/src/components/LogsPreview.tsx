import { useEffect, useState } from "react";
import { GetAppLogs } from "../../wailsjs/go/main/App";
import type { AppLogEntry } from "../types";

const LIMIT = 100;

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

export default function LogsPreview() {
  const [entries, setEntries] = useState<AppLogEntry[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    GetAppLogs(LIMIT)
      .then((data) => setEntries(data ?? []))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="settings-logs-section">
      <div className="detail-section-header">App Logs</div>

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
    </div>
  );
}
