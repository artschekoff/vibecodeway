import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { X, Plus, Activity } from "lucide-react";
import { TaskManagerModal } from "./TaskManagerModal";
import { Terminal } from "./Terminal";
import { TerminalWrite, AgentResumeCommand, ListAIPrompts } from "../../../wailsjs/go/main/App";
import { useTerminalCommands } from "../../hooks/useTerminalCommands";
import { applyPlaceholders, withAgentHydration } from "../../lib/terminalPlaceholders";
import { matchTerminalHotkey } from "../../lib/terminalHotkeys";
import { useHotkey } from "../../hotkeys/HotkeyProvider";
import { useTabVisible } from "../../contexts/TabVisibleContext";
import { loadTerminals, saveTerminals } from "../../lib/terminalSession";
import { registerTermSession, unregisterTermSession } from "../../lib/terminalRegistry";
import { FOCUS_TERMINAL_EVENT } from "../../lib/terminalAttention";

// Stable local ids, independent of cwd, so two terminals in the same folder are
// distinct and neither remounts when the list changes.
let seq = 0;
const nextKey = () => `t${++seq}`;

const basename = (p: string) => p.split("/").filter(Boolean).pop() ?? p;

// Insert-command picker styles (a custom popover, since a native <select> can't
// be opened programmatically in WebKit). Mirrors EditorSettingsMenu's popover.
const insertTrigger: React.CSSProperties = {
  fontSize: 11, background: "none", border: "1px solid var(--border)", whiteSpace: "nowrap",
  borderRadius: 4, color: "var(--text-muted)", cursor: "pointer", padding: "1px 6px",
};
const insertPopover: React.CSSProperties = {
  position: "fixed", zIndex: 50,
  minWidth: 180, maxHeight: 260, overflowY: "auto",
  background: "var(--bg-panel, var(--bg))", border: "1px solid var(--border)",
  borderRadius: 6, padding: 4, display: "flex", flexDirection: "column", gap: 2,
  boxShadow: "0 6px 20px rgba(0,0,0,0.3)", fontSize: 11, color: "var(--text)",
};
const insertItem: React.CSSProperties = {
  textAlign: "left", background: "none", border: "none", color: "var(--text)",
  cursor: "pointer", padding: "3px 8px", borderRadius: 4, fontSize: 11, whiteSpace: "nowrap",
};

// A tab candidate for the per-poll resume-command reduction below. Only the
// fields the reduction needs, so it stays testable without the rest of a
// terminal's state.
export interface ResumeCandidate {
  key: string;
  cwd: string;
  resume?: string;
}

// Reduces one poll's raw AgentResumeCommand results so that within any single
// cwd, at most one tab ends up holding a given resume command — otherwise two
// tabs opened in the same folder (one running an agent, one a plain shell for
// git/tests) both receive the same command and both auto-launch it on the next
// restore.
//
// Ownership is sticky: a tab that already holds a command keeps it forever, so
// a tab opened later in the same folder can never inherit a session another
// tab is already resuming. Only when nobody at that cwd owns the command yet
// is it handed to exactly one tab — the earliest in `tabs` order.
//
// Returns one entry per input tab: the command that tab should be assigned
// from this poll, or "" for "nothing from this poll" (never clears an
// existing `resume` — the caller's truthy guard handles that).
//
// ponytail: this cannot tell two tabs at the *same* cwd apart when both
// genuinely ran their own agent conversation — only one gets restored, the
// other comes back a plain shell. Exact per-terminal attribution needs a
// Claude Code SessionStart hook keyed by a per-PTY env var (what cmux
// installs, and the plan's documented upgrade path), not a heuristic here.
export function assignResumePerFolder(tabs: ResumeCandidate[], found: string[]): string[] {
  const result = tabs.map((_, i) => found[i] || "");

  const indicesByCwd = new Map<string, number[]>();
  tabs.forEach((t, i) => {
    const arr = indicesByCwd.get(t.cwd);
    if (arr) arr.push(i);
    else indicesByCwd.set(t.cwd, [i]);
  });

  for (const indices of indicesByCwd.values()) {
    const commands = new Set(indices.map((i) => result[i]).filter(Boolean));
    for (const cmd of commands) {
      const owner = indices.find((i) => (tabs[i].resume ?? "") === cmd);
      const winner = owner ?? indices.find((i) => result[i] === cmd);
      for (const i of indices) {
        if (result[i] === cmd && i !== winner) result[i] = "";
      }
    }
  }
  return result;
}

// Tab title: repo name first, then the folder name (usually the branch/worktree).
// A worktree lives at `<repo>/.worktrees/<branch>`, so the repo is the segment
// before ".worktrees". For a plain checkout the two coincide — show it once.
export function tabLabel(cwd: string): string {
  const parts = cwd.split("/").filter(Boolean);
  const name = parts[parts.length - 1] ?? cwd;
  const wt = parts.indexOf(".worktrees");
  const repo = wt > 0 ? parts[wt - 1] : name;
  return repo === name ? name : `${repo} / ${name}`;
}

/**
 * Holds one or more terminals for the editor. Every terminal stays mounted (the
 * inactive ones are display:none) so its PTY session and scrollback survive tab
 * switches. A terminal's PTY is closed only when it is removed via its × button.
 * New terminals open in the active terminal's folder.
 */
export function TerminalPanel({ spawnCwd, contextName, pendingInsert, pendingOpen, persistKey, onActiveCwd }: { spawnCwd: string; contextName?: string; pendingInsert?: { text: string; id: number }; pendingOpen?: { cwd: string; id: number }; persistKey?: string; onActiveCwd?: (cwd: string) => void }) {
  const [terminals, setTerminals] = useState<{ key: string; cwd: string; openedAt: number; resume?: string }[]>([]);
  const [activeKey, setActiveKey] = useState<string>("");
  const seededRef = useRef(false);
  const sessionByKey = useRef<Map<string, string>>(new Map());
  // Resume commands owed to restored tabs, drained as each tab's PTY comes up.
  // A ref rather than state: it must fire exactly once per tab, and re-renders
  // must not replay it.
  const pendingResume = useRef<Map<string, string>>(new Map());
  const wrapperByKey = useRef<Map<string, HTMLDivElement>>(new Map());
  // Tab chips by key: with many tabs the strip scrolls horizontally, and a
  // newly opened or activated tab can sit outside the scrolled viewport —
  // scroll it back into view whenever the active tab changes.
  const tabElByKey = useRef<Map<string, HTMLDivElement>>(new Map());
  useEffect(() => {
    tabElByKey.current.get(activeKey)?.scrollIntoView?.({ inline: "nearest", block: "nearest" });
  }, [activeKey]);
  const rootRef = useRef<HTMLDivElement>(null);
  const insertMenuRef = useRef<HTMLDivElement>(null);
  const insertBtnRef = useRef<HTMLButtonElement>(null);
  const [insertOpen, setInsertOpen] = useState(false);
  const [taskManagerOpen, setTaskManagerOpen] = useState(false);
  const [insertPos, setInsertPos] = useState<{ left: number; top: number } | null>(null);
  // Keyboard-navigable active row. State-driven (not DOM focus) so ↑/↓/Enter work
  // regardless of what holds focus — a native <button>.focus() doesn't stick in
  // WebKit on click, and the shell/xterm would otherwise eat the keys.
  const [insertActive, setInsertActive] = useState(0);
  const insertActiveRef = useRef(0);
  insertActiveRef.current = insertActive;
  const lastInsertId = useRef(0);
  const { commands } = useTerminalCommands();
  // Agent hydration prompt body, loaded once; empty string = feature off.
  const hydrationPromptRef = useRef("");
  useEffect(() => {
    // try/catch: unit-test mocks of the wailsjs module may not stub this
    // binding; hydration simply stays off there.
    try {
      void ListAIPrompts()
        .then((rows) => { hydrationPromptRef.current = rows?.find((p) => p.id === "agent_hydration")?.body ?? ""; })
        .catch(() => {});
    } catch { /* binding absent */ }
  }, []);

  const focusTerminal = (key: string) => {
    wrapperByKey.current.get(key)
      ?.querySelector<HTMLTextAreaElement>(".xterm-helper-textarea")
      ?.focus();
  };

  const setSession = (key: string, cwd: string, sessionId: string) => {
    if (!sessionId) {
      const old = sessionByKey.current.get(key);
      if (old) unregisterTermSession(old);
      sessionByKey.current.delete(key);
      return;
    }
    sessionByKey.current.set(key, sessionId);
    registerTermSession(sessionId, { contextKey: contextName ?? "", contextId: persistKey, tabLabel: tabLabel(cwd), cwd });
    // A restored agent tab types its resume command as soon as its shell exists.
    // Writing immediately is safe: the tty buffers the line until the shell has
    // finished sourcing its rc files and starts reading input.
    const resume = pendingResume.current.get(key);
    if (resume) {
      pendingResume.current.delete(key);
      void TerminalWrite(sessionId, resume + "\r");
    }
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
    // Hydration prompt (editable in Prompts settings, id "agent_hydration"):
    // becomes the agent's first argument so it boots primed with this context.
    // Newlines are flattened — a raw \n would submit the command mid-typing.
    const hydration = applyPlaceholders(hydrationPromptRef.current, { "context-name": contextName ?? "" })
      .replace(/\s*\n\s*/g, " ").trim();
    const substituted = applyPlaceholders(command, { "context-name": contextName ?? "" });
    const text = withAgentHydration(substituted, hydration);
    // Append "\r" (Enter) only when the snippet opts in, otherwise just type it.
    if (text) void TerminalWrite(sid, sendEnter ? text + "\r" : text);
    focusActiveTerminal();
  };

  // Bring back the tabs this editor had last time; otherwise seed one at the
  // repo root. Runs once, as soon as a repo path is known.
  useEffect(() => {
    if (seededRef.current || !spawnCwd) return;
    seededRef.current = true;
    const saved = persistKey ? loadTerminals(persistKey) : [];
    const restored: { key: string; cwd: string; openedAt: number; resume?: string }[] = saved.length
      ? saved.map((t) => ({ key: nextKey(), cwd: t.cwd, openedAt: t.openedAt, resume: t.resume }))
      : [{ key: nextKey(), cwd: spawnCwd, openedAt: Date.now() }];
    setTerminals(restored);
    setActiveKey(restored[0].key);
    for (const t of restored) {
      if (t.resume) pendingResume.current.set(t.key, t.resume);
    }
  }, [spawnCwd, persistKey]);

  // Mirror the tab list into storage on every change, so a crash or a force-quit
  // still leaves the last known layout behind.
  useEffect(() => {
    if (!persistKey || !seededRef.current) return;
    saveTerminals(persistKey, terminals.map(({ cwd, openedAt, resume }) => ({ cwd, openedAt, resume })));
  }, [persistKey, terminals]);

  // Ask the backend which tabs have a coding-agent conversation on disk, and
  // remember the command that resumes it. Polled instead of computed on close,
  // because a force-quit never runs an unmount handler. Paused while the tab
  // is hidden — N background editor tabs each scanning transcripts every 15s
  // adds up.
  const resumePollVisible = useTabVisible();
  useEffect(() => {
    if (!persistKey || terminals.length === 0 || !resumePollVisible) return;
    let cancelled = false;
    const refresh = async () => {
      const found = await Promise.all(
        terminals.map((t) => AgentResumeCommand(t.cwd, t.openedAt).catch(() => "")),
      );
      if (cancelled) return;
      setTerminals((ts) => {
        // Bail out by identity when nothing changed, so this effect's own
        // updates cannot retrigger it forever. A falsy poll result never
        // counts as a change (mirrors the truthy-guard below) — otherwise a
        // tab whose resume is already known would flip-flop against a "" result
        // forever, rebuilding the array every tick without ever converging.
        if (ts.length !== found.length) return ts;
        // Scope assignments per folder first, so two tabs sharing a cwd never
        // both end up auto-launching the same agent conversation.
        const assigned = assignResumePerFolder(ts, found);
        const changed = ts.some((t, i) => assigned[i] && assigned[i] !== (t.resume ?? ""));
        if (!changed) return ts;
        return ts.map((t, i) => (assigned[i] ? { ...t, resume: assigned[i] } : t));
      });
    };
    void refresh();
    const h = setInterval(() => void refresh(), 15000);
    return () => { cancelled = true; clearInterval(h); };
  }, [persistKey, terminals, resumePollVisible]);

  const openTerminalAt = (cwd: string) => {
    if (!cwd) return;
    const key = nextKey();
    setTerminals((ts) => [...ts, { key, cwd, openedAt: Date.now() }]);
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
    const closed = terminals.find((t) => t.key === key);
    const remaining = terminals.filter((t) => t.key !== key);
    setTerminals(remaining);
    if (key === activeKey) {
      // Fall back within the SAME repo only — tabs of other repos are hidden by
      // the repo filter, and activating one would silently flip the selector.
      const pool = remaining.filter((t) => t.cwd === closed?.cwd);
      const fallback = pool[pool.length - 1];
      setActiveKey(fallback?.key ?? "");
      if (fallback) setTimeout(() => focusTerminal(fallback.key), 0);
    }
  };

  // ── Tabs are bound to the repo selector (two-way) ──
  // Only the selected repo's tabs are shown; the others stay mounted (PTYs
  // keep running) but hidden.
  const visibleTerminals = spawnCwd ? terminals.filter((t) => t.cwd === spawnCwd) : terminals;
  // Selector changed → land on that repo's tab (or none, if it has no tabs).
  // prevSpawnCwd guards the mount run: reacting there would race the seeding
  // effect's setActiveKey (its setState hasn't applied yet) and clobber it.
  const prevSpawnCwd = useRef(spawnCwd);
  useEffect(() => {
    if (prevSpawnCwd.current === spawnCwd) return;
    prevSpawnCwd.current = spawnCwd;
    // Empty list = the seeding effect is populating in this same pass (its
    // setState hasn't applied yet) — let it pick the active tab.
    if (!spawnCwd || terminals.length === 0) return;
    const active = terminals.find((t) => t.key === activeKey);
    if (active?.cwd === spawnCwd) return;
    setActiveKey(terminals.find((t) => t.cwd === spawnCwd)?.key ?? "");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [spawnCwd]);
  // Activated a tab of another repo (e.g. "Open terminal here" from the file
  // tree) → tell the parent so the repo selector follows.
  useEffect(() => {
    const active = terminals.find((t) => t.key === activeKey);
    if (active && spawnCwd && active.cwd !== spawnCwd) onActiveCwd?.(active.cwd);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeKey]);

  // Switch active terminal and move keyboard focus into it (used by hotkeys).
  const switchTo = (key: string) => {
    setActiveKey(key);
    setTimeout(() => focusTerminal(key), 0);
  };
  // "Jump to the waiting terminal" (toast / hotkey): activate the tab that
  // owns the session; focusing xterm then clears the waiting mark.
  useEffect(() => {
    const onFocus = (e: Event) => {
      const sid = (e as CustomEvent<{ sessionId: string }>).detail?.sessionId;
      for (const [key, id] of sessionByKey.current) if (id === sid) { switchTo(key); return; }
    };
    window.addEventListener(FOCUS_TERMINAL_EVENT, onFocus);
    return () => window.removeEventListener(FOCUS_TERMINAL_EVENT, onFocus);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const cycleTerminal = (dir: 1 | -1) => {
    const pool = visibleTerminals;
    const n = pool.length;
    if (n === 0) return;
    const idx = pool.findIndex((t) => t.key === activeKey);
    switchTo(pool[(idx + dir + n) % n].key);
  };

  // Rebindable terminal commands via the hotkey layer: ⌘T new, ⌘⇧T here, ⌘I
  // insert, ⌘] / ⌘[ next / prev. All "global" — scoping them to terminal DOM
  // focus made them dead unless the cursor was already inside xterm. The panel
  // just needs to be mounted. The residual capture listener below still owns
  // ⌘W and ⌃1-9.
  // `enabled: tabVisible` — every open editor tab keeps its TerminalPanel
  // mounted (hidden ones via display:none), and the dispatcher picks the first
  // registered handler; without the gate these fired in a hidden tab.
  const tabVisible = useTabVisible();
  useHotkey("terminal.new", () => addTerminal(), { scope: "global", enabled: tabVisible });
  useHotkey("terminal.here", () => openTerminalAt(spawnCwd), { scope: "global", enabled: tabVisible });
  // A native <select> can't be opened programmatically in WebKit (Wails' macOS
  // webview): showPicker() is Chromium-only. So the picker is a React
  // state-driven popover the hotkey just flips open.
  useHotkey("terminal.insert", () => setInsertOpen(true), { scope: "global", enabled: tabVisible });
  useHotkey("terminal.next", () => cycleTerminal(1), { scope: "global", enabled: tabVisible });
  useHotkey("terminal.prev", () => cycleTerminal(-1), { scope: "global", enabled: tabVisible });

  // The toolbar row is `overflow-x: auto`, which per CSS also clips vertically —
  // an absolutely-positioned popover inside it would be cut to the row's height.
  // So anchor the menu with position:fixed to the trigger's viewport rect,
  // measured before paint to avoid a flash.
  useLayoutEffect(() => {
    if (!insertOpen) return;
    const r = insertBtnRef.current?.getBoundingClientRect();
    if (r) setInsertPos({ left: r.left, top: r.bottom + 4 });
  }, [insertOpen]);

  // The picker is keyboard-driven while open: a capture-phase window listener owns
  // ↑/↓/Enter/Escape so they can't leak into the shell (xterm), and drives the
  // active row via state. Reset to the top each time it opens.
  useEffect(() => { if (insertOpen) setInsertActive(0); }, [insertOpen]);
  useEffect(() => {
    if (!insertOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setInsertActive((i) => Math.min(i + 1, commands.length - 1)); }
      else if (e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setInsertActive((i) => Math.max(i - 1, 0)); }
      else if (e.key === "Enter") {
        e.preventDefault(); e.stopPropagation();
        const c = commands[insertActiveRef.current];
        if (c) { insertCommand(c.command, c.sendEnter); setInsertOpen(false); }
      } else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); setInsertOpen(false); focusActiveTerminal(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [insertOpen, commands]);
  // Keep the active row visible when navigating a long list.
  useEffect(() => {
    if (!insertOpen) return;
    insertMenuRef.current?.querySelectorAll<HTMLButtonElement>("[role='menuitem']")[insertActive]?.scrollIntoView?.({ block: "nearest" });
  }, [insertActive, insertOpen]);

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
    <div ref={rootRef}
      style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      {/* Insert and + are pinned via flex `order` (insert -1, tabs strip 1): they
          keep one spot however many tabs open; only the tabs strip scrolls. */}
      <div style={{ display: "flex", alignItems: "center", gap: 2, padding: "2px 4px", borderBottom: "1px solid var(--border)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 2, flex: 1, minWidth: 0, overflowX: "auto", order: 1 }}>
        {visibleTerminals.map((t) => (
          <div
            key={t.key}
            ref={(el) => { if (el) tabElByKey.current.set(t.key, el); else tabElByKey.current.delete(t.key); }}
            onClick={() => setActiveKey(t.key)}
            title={t.cwd}
            style={{
              display: "flex", alignItems: "center", gap: 4, padding: "2px 6px", fontSize: 11,
              cursor: "pointer", borderRadius: 4, whiteSpace: "nowrap",
              background: t.key === activeKey ? "var(--bg-hover, rgba(127,127,127,0.15))" : "transparent",
              color: t.key === activeKey ? "var(--text)" : "var(--text-muted)",
            }}
          >
            <span>{tabLabel(t.cwd)}</span>
            <button
              onClick={(e) => { e.stopPropagation(); closeTerminal(t.key); }}
              aria-label={`close terminal ${basename(t.cwd)}`}
              style={{ display: "flex", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 0 }}
            >
              <X size={11} />
            </button>
          </div>
        ))}
        </div>
        <button
          onClick={addTerminal}
          aria-label="new terminal"
          title="New terminal"
          style={{ display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: "0 4px", flexShrink: 0 }}
        >
          <Plus size={13} />
        </button>
        <button
          onClick={() => setTaskManagerOpen(true)}
          aria-label="task manager"
          title="Task Manager (per-tab CPU / RAM)"
          style={{ display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: "0 4px", flexShrink: 0 }}
        >
          <Activity size={13} />
        </button>
        <div style={{ position: "relative", flexShrink: 0, order: -1 }}>
          <button
            ref={insertBtnRef}
            aria-label="Insert terminal command"
            aria-haspopup="menu"
            aria-expanded={insertOpen}
            title="Insert command"
            onClick={() => setInsertOpen((o) => !o)}
            style={insertTrigger}
          >
            Insert command…
          </button>
          {insertOpen && insertPos && (
            <>
              {/* click-away */}
              <div onClick={() => setInsertOpen(false)} style={{ position: "fixed", inset: 0, zIndex: 40 }} />
              <div
                ref={insertMenuRef}
                role="menu"
                style={{ ...insertPopover, left: insertPos.left, top: insertPos.top }}
              >
                {commands.length === 0 && (
                  <div style={{ padding: "3px 8px", color: "var(--text-muted)" }}>No commands</div>
                )}
                {commands.map((c, i) => (
                  <button
                    key={c.id}
                    role="menuitem"
                    aria-selected={i === insertActive}
                    style={{ ...insertItem, background: i === insertActive ? "var(--bg-hover, rgba(127,127,127,0.15))" : "none" }}
                    onMouseEnter={() => setInsertActive(i)}
                    onClick={() => { insertCommand(c.command, c.sendEnter); setInsertOpen(false); }}
                  >
                    {c.title}
                  </button>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
      <div style={{ flex: 1, minHeight: 0, position: "relative" }}>
        {terminals.map((t) => (
          <div key={t.key}
            ref={(el) => { if (el) wrapperByKey.current.set(t.key, el); else wrapperByKey.current.delete(t.key); }}
            style={{ position: "absolute", inset: 0, display: t.key === activeKey ? "block" : "none" }}>
            {/* `exit` in the shell retires the tab with it — a dead terminal is
                nothing to keep a tab for. */}
            <Terminal cwd={t.cwd} onSession={(id) => setSession(t.key, t.cwd, id)} onExit={() => closeTerminal(t.key)} />
          </div>
        ))}
        {visibleTerminals.length === 0 && (
          <div style={{ fontSize: 12, padding: 12 }}>{spawnCwd ? "No terminal for this repo — press + to open one." : "No repo path on disk."}</div>
        )}
      </div>
      {taskManagerOpen && <TaskManagerModal onClose={() => setTaskManagerOpen(false)} />}
    </div>
  );
}
