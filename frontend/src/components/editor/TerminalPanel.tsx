import { useEffect, useRef, useState } from "react";
import { X, Plus } from "lucide-react";
import { Terminal } from "./Terminal";
import { TerminalWrite } from "../../../wailsjs/go/main/App";
import { useTerminalCommands } from "../../hooks/useTerminalCommands";
import { applyPlaceholders } from "../../lib/terminalPlaceholders";

// Stable local ids, independent of cwd, so two terminals in the same folder are
// distinct and neither remounts when the list changes.
let seq = 0;
const nextKey = () => `t${++seq}`;

const basename = (p: string) => p.split("/").filter(Boolean).pop() ?? p;

/**
 * Holds one or more terminals for the editor. Every terminal stays mounted (the
 * inactive ones are display:none) so its PTY session and scrollback survive tab
 * switches. A terminal's PTY is closed only when it is removed via its × button.
 * New terminals open in the active terminal's folder.
 */
export function TerminalPanel({ spawnCwd, contextName }: { spawnCwd: string; contextName?: string }) {
  const [terminals, setTerminals] = useState<{ key: string; cwd: string }[]>([]);
  const [activeKey, setActiveKey] = useState<string>("");
  const seededRef = useRef(false);
  const sessionByKey = useRef<Map<string, string>>(new Map());
  const { commands } = useTerminalCommands();

  const setSession = (key: string, sessionId: string) => {
    if (sessionId) sessionByKey.current.set(key, sessionId);
    else sessionByKey.current.delete(key);
  };

  const insertCommand = (command: string) => {
    const sid = sessionByKey.current.get(activeKey);
    if (!sid) return;
    const text = applyPlaceholders(command, { "context-name": contextName ?? "" });
    if (text) void TerminalWrite(sid, text); // type only — never append Enter
  };

  // Auto-create the first terminal once a repo path is known (once).
  useEffect(() => {
    if (seededRef.current || !spawnCwd) return;
    seededRef.current = true;
    const key = nextKey();
    setTerminals([{ key, cwd: spawnCwd }]);
    setActiveKey(key);
  }, [spawnCwd]);

  const addTerminal = () => {
    const activeCwd = terminals.find((t) => t.key === activeKey)?.cwd ?? spawnCwd;
    if (!activeCwd) return;
    const key = nextKey();
    setTerminals((ts) => [...ts, { key, cwd: activeCwd }]);
    setActiveKey(key);
  };

  const closeTerminal = (key: string) => {
    setTerminals((ts) => {
      const idx = ts.findIndex((t) => t.key === key);
      const next = ts.filter((t) => t.key !== key);
      if (key === activeKey) {
        const fallback = next[idx] ?? next[idx - 1] ?? next[0];
        setActiveKey(fallback?.key ?? "");
      }
      return next;
    });
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 2, padding: "2px 4px", borderBottom: "1px solid var(--border)", overflowX: "auto" }}>
        {terminals.map((t) => (
          <div
            key={t.key}
            onClick={() => setActiveKey(t.key)}
            title={t.cwd}
            style={{
              display: "flex", alignItems: "center", gap: 4, padding: "2px 6px", fontSize: 11,
              cursor: "pointer", borderRadius: 4, whiteSpace: "nowrap",
              background: t.key === activeKey ? "var(--bg-hover, rgba(127,127,127,0.15))" : "transparent",
              color: t.key === activeKey ? "var(--text)" : "var(--text-muted)",
            }}
          >
            <span>{basename(t.cwd)}</span>
            <button
              onClick={(e) => { e.stopPropagation(); closeTerminal(t.key); }}
              aria-label={`close terminal ${basename(t.cwd)}`}
              style={{ display: "flex", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 0 }}
            >
              <X size={11} />
            </button>
          </div>
        ))}
        <button
          onClick={addTerminal}
          aria-label="new terminal"
          title="New terminal"
          style={{ display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: "0 4px" }}
        >
          <Plus size={13} />
        </button>
        <select
          aria-label="Insert terminal command"
          value=""
          onChange={(e) => {
            const c = commands.find((cmd) => cmd.id === e.target.value);
            if (c) insertCommand(c.command);
            e.currentTarget.selectedIndex = 0;
          }}
          style={{ fontSize: 11, marginLeft: 8 }}
        >
          <option value="">Insert command…</option>
          {commands.map((c) => <option key={c.id} value={c.id}>{c.title}</option>)}
        </select>
      </div>
      <div style={{ flex: 1, minHeight: 0, position: "relative" }}>
        {terminals.map((t) => (
          <div key={t.key} style={{ position: "absolute", inset: 0, display: t.key === activeKey ? "block" : "none" }}>
            <Terminal cwd={t.cwd} onSession={(id) => setSession(t.key, id)} />
          </div>
        ))}
        {terminals.length === 0 && <div style={{ fontSize: 12, padding: 12 }}>No terminal.</div>}
      </div>
    </div>
  );
}
