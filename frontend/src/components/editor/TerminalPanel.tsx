import { useEffect, useRef, useState } from "react";
import { X, Plus } from "lucide-react";
import { Terminal } from "./Terminal";
import { TerminalWrite } from "../../../wailsjs/go/main/App";
import { useTerminalCommands } from "../../hooks/useTerminalCommands";
import { applyPlaceholders } from "../../lib/terminalPlaceholders";
import { matchTerminalHotkey } from "../../lib/terminalHotkeys";

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
export function TerminalPanel({ spawnCwd, contextName, pendingInsert, pendingOpen }: { spawnCwd: string; contextName?: string; pendingInsert?: { text: string; id: number }; pendingOpen?: { cwd: string; id: number } }) {
  const [terminals, setTerminals] = useState<{ key: string; cwd: string }[]>([]);
  const [activeKey, setActiveKey] = useState<string>("");
  const seededRef = useRef(false);
  const sessionByKey = useRef<Map<string, string>>(new Map());
  const wrapperByKey = useRef<Map<string, HTMLDivElement>>(new Map());
  const rootRef = useRef<HTMLDivElement>(null);
  const lastInsertId = useRef(0);
  const { commands } = useTerminalCommands();

  const focusTerminal = (key: string) => {
    wrapperByKey.current.get(key)
      ?.querySelector<HTMLTextAreaElement>(".xterm-helper-textarea")
      ?.focus();
  };

  const setSession = (key: string, sessionId: string) => {
    if (sessionId) sessionByKey.current.set(key, sessionId);
    else sessionByKey.current.delete(key);
  };

  // Move keyboard focus into the active xterm so the user can immediately edit /
  // press Enter after text is inserted (otherwise focus stays on the palette
  // <select>). `.xterm-helper-textarea` is xterm's hidden input; optional-chained
  // so it's a no-op under the test mock.
  const focusActiveTerminal = () => {
    wrapperByKey.current.get(activeKey)
      ?.querySelector<HTMLTextAreaElement>(".xterm-helper-textarea")
      ?.focus();
  };

  // Type externally-requested text (e.g. "Ask AI" from the diff editor) into the
  // active terminal — never appends Enter, so the user reviews before sending.
  useEffect(() => {
    if (!pendingInsert || pendingInsert.id === lastInsertId.current) return;
    lastInsertId.current = pendingInsert.id;
    const sid = sessionByKey.current.get(activeKey);
    if (sid && pendingInsert.text) {
      void TerminalWrite(sid, pendingInsert.text);
      focusActiveTerminal();
    }
  }, [pendingInsert, activeKey]);

  const insertCommand = (command: string, sendEnter: boolean) => {
    const sid = sessionByKey.current.get(activeKey);
    if (!sid) return;
    const text = applyPlaceholders(command, { "context-name": contextName ?? "" });
    // Append "\r" (Enter) only when the snippet opts in, otherwise just type it.
    if (text) void TerminalWrite(sid, sendEnter ? text + "\r" : text);
    focusActiveTerminal();
  };

  // Auto-create the first terminal once a repo path is known (once).
  useEffect(() => {
    if (seededRef.current || !spawnCwd) return;
    seededRef.current = true;
    const key = nextKey();
    setTerminals([{ key, cwd: spawnCwd }]);
    setActiveKey(key);
  }, [spawnCwd]);

  const openTerminalAt = (cwd: string) => {
    if (!cwd) return;
    const key = nextKey();
    setTerminals((ts) => [...ts, { key, cwd }]);
    setActiveKey(key);
    setTimeout(() => focusTerminal(key), 0); // focus once the new terminal mounts
  };

  const addTerminal = () => {
    const activeCwd = terminals.find((t) => t.key === activeKey)?.cwd ?? spawnCwd;
    openTerminalAt(activeCwd);
  };

  // Spawn a terminal at an externally-requested folder (e.g. "Open terminal here"
  // on a repo in the changed-files panel).
  const lastOpenId = useRef(0);
  useEffect(() => {
    if (!pendingOpen || pendingOpen.id === lastOpenId.current) return;
    lastOpenId.current = pendingOpen.id;
    openTerminalAt(pendingOpen.cwd);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingOpen]);

  const closeTerminal = (key: string) => {
    const idx = terminals.findIndex((t) => t.key === key);
    const remaining = terminals.filter((t) => t.key !== key);
    setTerminals(remaining);
    if (key === activeKey) {
      const fallback = remaining[idx] ?? remaining[idx - 1] ?? remaining[0];
      setActiveKey(fallback?.key ?? "");
      if (fallback) setTimeout(() => focusTerminal(fallback.key), 0);
    }
  };

  // Switch active terminal and move keyboard focus into it (used by hotkeys).
  const switchTo = (key: string) => {
    setActiveKey(key);
    setTimeout(() => focusTerminal(key), 0);
  };
  const cycleTerminal = (dir: 1 | -1) => {
    const n = terminals.length;
    if (n === 0) return;
    const idx = terminals.findIndex((t) => t.key === activeKey);
    switchTo(terminals[(idx + dir + n) % n].key);
  };

  // cmux "surface" keymap for the terminal tabs. Capture phase on the panel root
  // runs before xterm's textarea listener; stopPropagation keeps the keys out of
  // the shell and preventDefault blocks webview defaults (e.g. ⌘W closing a tab).
  // Re-bound on state change so the handler sees the current terminals/activeKey.
  useEffect(() => {
    const el = rootRef.current;
    if (!el) return;
    const onKey = (e: KeyboardEvent) => {
      const hk = matchTerminalHotkey(e);
      if (!hk) return;
      e.preventDefault();
      e.stopPropagation();
      switch (hk.kind) {
        case "new": addTerminal(); break;
        case "close": if (activeKey) closeTerminal(activeKey); break;
        case "next": cycleTerminal(1); break;
        case "prev": cycleTerminal(-1); break;
        case "jump": {
          const t = hk.index === -1 ? terminals[terminals.length - 1] : terminals[hk.index];
          if (t) switchTo(t.key);
          break;
        }
      }
    };
    el.addEventListener("keydown", onKey, true);
    return () => el.removeEventListener("keydown", onKey, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [terminals, activeKey, spawnCwd]);

  return (
    <div ref={rootRef} style={{ display: "flex", flexDirection: "column", height: "100%" }}>
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
            if (c) insertCommand(c.command, c.sendEnter);
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
          <div key={t.key}
            ref={(el) => { if (el) wrapperByKey.current.set(t.key, el); else wrapperByKey.current.delete(t.key); }}
            style={{ position: "absolute", inset: 0, display: t.key === activeKey ? "block" : "none" }}>
            <Terminal cwd={t.cwd} onSession={(id) => setSession(t.key, id)} />
          </div>
        ))}
        {terminals.length === 0 && <div style={{ fontSize: 12, padding: 12 }}>No terminal.</div>}
      </div>
    </div>
  );
}
