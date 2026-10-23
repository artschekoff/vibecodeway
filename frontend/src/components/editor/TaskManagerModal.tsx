import { Fragment, useEffect, useState } from "react";
import { X } from "lucide-react";
import { GetProcessStats } from "../../../wailsjs/go/main/App";
import { termSessionInfo } from "../../lib/terminalRegistry";

const POLL_MS = 2000;

type SessionStat = { sessionId: string; pid: number; cpu: number; rssBytes: number; procs: number; topComm: string };
type Stats = { app: SessionStat; sessions: SessionStat[] };

type Row = SessionStat & { contextKey: string; tabLabel: string };

function fmtMem(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${Math.round(mb)} MB`;
}

// groupRows labels backend session stats with their owning context/tab and
// groups them by context, contexts and rows both sorted by CPU descending.
export function groupRows(sessions: SessionStat[]): { contextKey: string; rows: Row[]; cpu: number; rss: number }[] {
  const rows: Row[] = sessions.map((s) => {
    const info = termSessionInfo(s.sessionId);
    return { ...s, contextKey: info?.contextKey || "(unknown)", tabLabel: info?.tabLabel || s.topComm || "terminal" };
  });
  const byContext = new Map<string, Row[]>();
  for (const r of rows) {
    const list = byContext.get(r.contextKey) ?? [];
    list.push(r);
    byContext.set(r.contextKey, list);
  }
  return [...byContext.entries()]
    .map(([contextKey, list]) => ({
      contextKey,
      rows: list.sort((a, b) => b.cpu - a.cpu),
      cpu: list.reduce((n, r) => n + r.cpu, 0),
      rss: list.reduce((n, r) => n + r.rssBytes, 0),
    }))
    .sort((a, b) => b.cpu - a.cpu);
}

const cell: React.CSSProperties = { padding: "3px 10px", fontSize: 11, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" };
const num: React.CSSProperties = { ...cell, textAlign: "right", fontVariantNumeric: "tabular-nums" };

export function TaskManagerModal({ onClose }: { onClose: () => void }) {
  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Row whose full (untruncated) names are shown in a details row below it:
  // "ctx:<contextKey>" for a group header, a sessionId for a session row.
  const [expanded, setExpanded] = useState<string | null>(null);
  const toggle = (key: string) => setExpanded((e) => (e === key ? null : key));

  useEffect(() => {
    let active = true;
    const poll = () => {
      GetProcessStats()
        .then((s) => { if (active) { setStats(s as Stats); setError(null); } })
        .catch((e) => { if (active) setError(e instanceof Error ? e.message : String(e)); });
    };
    poll();
    const t = window.setInterval(poll, POLL_MS);
    return () => { active = false; window.clearInterval(t); };
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") { e.stopPropagation(); onClose(); } };
    // Capture phase so a fullscreen editor's own Esc handler doesn't also fire.
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  const groups = stats ? groupRows(stats.sessions ?? []) : [];

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 120, background: "rgba(0,0,0,0.5)", display: "flex", alignItems: "center", justifyContent: "center" }}
      onClick={onClose}>
      <div role="dialog" aria-label="task manager"
        style={{ width: 560, maxHeight: "70vh", overflow: "auto", background: "var(--surface, #1e1e1e)", border: "1px solid var(--border)", borderRadius: 8, display: "flex", flexDirection: "column" }}
        onClick={(e) => e.stopPropagation()}>
        <div style={{ display: "flex", alignItems: "center", padding: "8px 12px", borderBottom: "1px solid var(--border)" }}>
          <span style={{ fontSize: 12, fontWeight: 600 }}>Task Manager</span>
          {stats && (
            <span style={{ fontSize: 11, color: "var(--text-muted)", marginLeft: 10 }}>
              app total: {stats.app.cpu.toFixed(1)}% CPU · {fmtMem(stats.app.rssBytes)} · {stats.app.procs} procs
            </span>
          )}
          <button aria-label="close task manager" onClick={onClose}
            style={{ marginLeft: "auto", display: "flex", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 2 }}>
            <X size={14} />
          </button>
        </div>
        {error && <div style={{ fontSize: 11, color: "var(--danger, #e5534b)", padding: "6px 12px" }}>{error}</div>}
        {!error && groups.length === 0 && (
          <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 16 }}>
            {stats ? "No terminal sessions running." : "Loading…"}
          </div>
        )}
        {groups.length > 0 && (
          // tableLayout:fixed + ellipsis cells keep the table inside the fixed
          // dialog width whatever the worktree/context names are; clicking a
          // row expands a details line with the full names.
          <table style={{ borderCollapse: "collapse", width: "100%", tableLayout: "fixed" }}>
            <thead>
              <tr style={{ color: "var(--text-muted)", textAlign: "left" }}>
                <th style={cell}>Tab</th>
                <th style={cell}>Top process</th>
                <th style={{ ...num, width: 52 }}>Procs</th>
                <th style={{ ...num, width: 56 }}>CPU %</th>
                <th style={{ ...num, width: 64 }}>RAM</th>
              </tr>
            </thead>
            {groups.map((g) => (
              <tbody key={g.contextKey}>
                <tr style={{ background: "var(--surface2, rgba(127,127,127,0.12))", cursor: "pointer" }}
                  title="Show full name" onClick={() => toggle("ctx:" + g.contextKey)}>
                  <td style={{ ...cell, fontWeight: 600 }} colSpan={3}>{g.contextKey}</td>
                  <td style={num}>{g.cpu.toFixed(1)}</td>
                  <td style={num}>{fmtMem(g.rss)}</td>
                </tr>
                {expanded === "ctx:" + g.contextKey && (
                  <tr>
                    <td colSpan={5} style={{ ...cell, whiteSpace: "normal", wordBreak: "break-all", color: "var(--text-muted)" }}>
                      {g.contextKey}
                    </td>
                  </tr>
                )}
                {g.rows.map((r) => (
                  <Fragment key={r.sessionId}>
                    <tr style={{ cursor: "pointer" }} title="Show full name"
                      onClick={() => toggle(r.sessionId)}>
                      <td style={{ ...cell, paddingLeft: 22 }}>{r.tabLabel}</td>
                      <td style={{ ...cell, color: "var(--text-muted)" }}>{r.topComm}</td>
                      <td style={num}>{r.procs}</td>
                      <td style={num}>{r.cpu.toFixed(1)}</td>
                      <td style={num}>{fmtMem(r.rssBytes)}</td>
                    </tr>
                    {expanded === r.sessionId && (
                      <tr>
                        <td colSpan={5} style={{ ...cell, paddingLeft: 22, whiteSpace: "normal", wordBreak: "break-all", color: "var(--text-muted)" }}>
                          {r.tabLabel} · {r.topComm}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            ))}
          </table>
        )}
      </div>
    </div>
  );
}
