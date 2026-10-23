// "Agent is waiting for input" state per PTY session, fed by Terminal.tsx
// (BEL / OSC 9 / OSC 777 from the agent) and consumed by the sidebar
// highlight, the global toast and the jump hotkey. Module-level like
// terminalRegistry: terminals of hidden tabs stay mounted and keep reporting.
import { useMemo, useSyncExternalStore } from "react";
import { termSessionInfo } from "./terminalRegistry";

export type WaitingTerminal = {
  sessionId: string;
  contextId?: string;
  contextKey: string;
  tabLabel: string;
  title: string;
  body: string;
  since: number;
  // The terminal was on screen when the signal came — the toast skips it,
  // the sidebar still marks it.
  visible: boolean;
};

const waiting = new Map<string, WaitingTerminal>();
let version = 0;
const listeners = new Set<() => void>();
function notify(): void {
  version++;
  listeners.forEach((cb) => cb());
}
function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  return () => { listeners.delete(cb); };
}

export function markWaiting(sessionId: string, title: string, body: string, visible: boolean): WaitingTerminal {
  const info = termSessionInfo(sessionId);
  const entry: WaitingTerminal = {
    sessionId, contextId: info?.contextId, contextKey: info?.contextKey ?? "",
    tabLabel: info?.tabLabel ?? "terminal", title, body, since: Date.now(), visible,
  };
  waiting.set(sessionId, entry);
  notify();
  return entry;
}

export function clearWaiting(sessionId: string): void {
  if (waiting.delete(sessionId)) notify();
}

export function waitingList(): WaitingTerminal[] {
  return [...waiting.values()].sort((a, b) => a.since - b.since);
}

export function useWaitingTerminals(): WaitingTerminal[] {
  const v = useSyncExternalStore(subscribe, () => version, () => version);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  return useMemo(() => waitingList(), [v]);
}

// --- preferences (per machine) ---
export type AttentionPrefs = { toast: boolean; native: boolean; highlight: boolean };
const PREFS_KEY = "vibecodeway:agent-attention";
const DEFAULTS: AttentionPrefs = { toast: true, native: true, highlight: true };
export function readPrefs(): AttentionPrefs {
  try { return { ...DEFAULTS, ...JSON.parse(localStorage.getItem(PREFS_KEY) ?? "{}") }; } catch { return { ...DEFAULTS }; }
}
export function writePrefs(p: AttentionPrefs): void {
  try { localStorage.setItem(PREFS_KEY, JSON.stringify(p)); } catch { /* storage unavailable */ }
}

// --- jump ---
// TerminalPanel listens for this and activates the tab owning the session.
export const FOCUS_TERMINAL_EVENT = "vibecodeway:focus-terminal";
export function requestFocusTerminal(sessionId: string): void {
  window.dispatchEvent(new CustomEvent(FOCUS_TERMINAL_EVENT, { detail: { sessionId } }));
}

type Tabs = { openEditorTab: (contextId: string, contextKey: string) => void; openAskTab: () => void };
export function jumpToTerminal(e: WaitingTerminal, tabs: Tabs): void {
  if (e.contextId) tabs.openEditorTab(e.contextId, e.contextKey);
  else tabs.openAskTab();
  // The target panel may be mounting; let React commit first.
  window.setTimeout(() => requestFocusTerminal(e.sessionId), 50);
}

// Cycle through waiting terminals, oldest first, continuing after the last jump.
let lastJump = "";
export function jumpToNextWaiting(tabs: Tabs): WaitingTerminal | null {
  const list = waitingList();
  if (list.length === 0) return null;
  const i = list.findIndex((e) => e.sessionId === lastJump);
  const next = list[(i + 1) % list.length];
  lastJump = next.sessionId;
  jumpToTerminal(next, tabs);
  return next;
}
