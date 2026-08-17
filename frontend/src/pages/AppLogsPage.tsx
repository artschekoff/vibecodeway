import { useEffect, useMemo, useState } from "react";
import { RefreshCw, Search, Trash2 } from "lucide-react";
import { GetAppLogs, ClearAppLogs } from "../../wailsjs/go/main/App";
import type { AppLogEntry } from "../types";
import { AppLogDetailModal } from "../components/AppLogDetailModal";

const LEVELS = [
  { value: "", label: "All" },
  { value: "debug", label: "Debug" },
  { value: "info", label: "Info" },
  { value: "warn", label: "Warn" },
  { value: "error", label: "Error" },
];

const SORTS = [
  { value: "date", label: "Date" },
  { value: "level", label: "Level" },
  { value: "caller", label: "Caller" },
] as const;

type SortKey = (typeof SORTS)[number]["value"];

// Higher rank surfaces first when sorting by level (most severe on top).
const LEVEL_RANK: Record<string, number> = { error: 4, warn: 3, info: 2, debug: 1 };

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
  const [sortBy, setSortBy] = useState<SortKey>("date");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<AppLogEntry | null>(null);
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

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = entries.filter((e) => {
      if (levelFilter && e.level !== levelFilter) return false;
      if (q) {
        const caller = (e.caller ?? "").toLowerCase();
        const msg = (e.msg ?? "").toLowerCase();
        if (!caller.includes(q) && !msg.includes(q)) return false;
      }
      return true;
    });
    return [...filtered].sort((a, b) => {
      if (sortBy === "level") {
        const d = (LEVEL_RANK[b.level] ?? 0) - (LEVEL_RANK[a.level] ?? 0);
        if (d !== 0) return d;
        return b.ts - a.ts;
      }
      if (sortBy === "caller") {
        const d = (a.caller ?? "").localeCompare(b.caller ?? "");
        if (d !== 0) return d;
        return b.ts - a.ts;
      }
      // date: newest first
      return b.ts - a.ts;
    });
  }, [entries, levelFilter, query, sortBy]);

  return (
    <div className="page">
      <div className="topbar">
        <div>
          <h1 style={{ fontSize: 16, margin: 0 }}>App Logs</h1>
        </div>
        <div className="settings-logs-search" style={{ flex: 1 }}>
          <Search size={14} className="settings-logs-search-icon" />
          <input
            type="search"
            placeholder="Search caller or message…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div className="settings-logs-filter-bar" style={{ margin: 0 }}>
          <label htmlFor="app-logs-sort">Sort</label>
          <select
            id="app-logs-sort"
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as SortKey)}
          >
            {SORTS.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
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
                <tr key={e.id} onClick={() => setSelected(e)} style={{ cursor: "pointer" }}>
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
        {selected && <AppLogDetailModal entry={selected} onClose={() => setSelected(null)} />}
      </div>
    </div>
  );
}
