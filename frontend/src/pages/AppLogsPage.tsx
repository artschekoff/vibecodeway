import { useEffect, useState } from "react";
import { RefreshCw, Trash2 } from "lucide-react";
import { GetAppLogs, ClearAppLogs } from "../../wailsjs/go/main/App";
import type { AppLogEntry } from "../types";

const LEVELS = [
  { value: "", label: "All" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

const LIMIT = 200;

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

export default function AppLogsPage() {
  const [entries, setEntries] = useState<AppLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [clearing, setClearing] = useState(false);
  const [confirmClear, setConfirmClear] = useState(false);
  const [levelFilter, setLevelFilter] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!confirmClear) return;
    const t = setTimeout(() => setConfirmClear(false), 3000);
    return () => clearTimeout(t);
  }, [confirmClear]);

  const load = () => {
    setLoading(true);
    setError("");
    GetAppLogs(LIMIT)
      .then((data) => setEntries(data ?? []))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, []);

  const handleClear = async () => {
    setClearing(true);
    try {
      await ClearAppLogs();
      setEntries([]);
    } catch (e) {
      setError(String(e));
    } finally {
      setClearing(false);
    }
  };

  const visible = levelFilter
    ? entries.filter((e) => e.level === levelFilter)
    : entries;

  return (
    <div className="page">
      <div className="topbar">
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 16, margin: 0 }}>App Logs</h1>
        </div>
        <div className="settings-logs-filter-bar" style={{ margin: 0 }}>
          <label htmlFor="app-logs-level">Level</label>
          <select
            id="app-logs-level"
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value)}
          >
            {LEVELS.map((l) => (
              <option key={l.value} value={l.value}>{l.label}</option>
            ))}
          </select>
        </div>
        <button className="btn-secondary" onClick={load} disabled={loading} title="Refresh">
          <RefreshCw size={14} />
        </button>
        <button
          className={confirmClear ? "btn-danger" : "btn-secondary"}
          onClick={() => {
            if (!confirmClear) { setConfirmClear(true); return; }
            setConfirmClear(false);
            void handleClear();
          }}
          disabled={clearing || entries.length === 0}
          title="Clear all logs"
        >
          <Trash2 size={14} /> {confirmClear ? "Confirm" : "Clear"}
        </button>
      </div>

      <div className="content">
        {error && <div className="error-banner">{error}</div>}

        {loading ? (
          <div className="settings-logs-empty">Loading…</div>
        ) : visible.length === 0 ? (
          <div className="settings-logs-empty">No log entries found.</div>
        ) : (
          <table className="settings-logs-table" style={{ marginTop: 12 }}>
            <thead>
              <tr>
                <th className="settings-logs-cell-date">Date</th>
                <th className="settings-logs-cell-level">Level</th>
                <th className="settings-logs-cell-context">Caller</th>
                <th className="settings-logs-cell-message">Message</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((e) => (
                <tr key={e.id}>
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
    </div>
  );
}
