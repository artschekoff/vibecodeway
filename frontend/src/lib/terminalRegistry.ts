// Maps live PTY session ids to the context/tab that owns them, so the Task
// Manager can label backend process stats. Module-level: TerminalPanels of
// hidden context tabs stay mounted, so every live session is registered.
export type TermSessionInfo = { contextKey: string; contextId?: string; tabLabel: string; cwd?: string };

const sessions = new Map<string, TermSessionInfo>();

// Change subscription so React surfaces (repo tiles) can re-render when
// terminals start/exit. Version counter doubles as the store snapshot for
// useSyncExternalStore.
let version = 0;
const listeners = new Set<() => void>();
function notify(): void {
  version++;
  listeners.forEach((cb) => cb());
}
export function subscribeTermSessions(cb: () => void): () => void {
  listeners.add(cb);
  return () => { listeners.delete(cb); };
}
export function termSessionsVersion(): number {
  return version;
}

export function registerTermSession(id: string, info: TermSessionInfo): void {
  // Identity guard: components may re-register the same session on every
  // render; notifying then would loop render → notify → render forever.
  const prev = sessions.get(id);
  if (prev && prev.contextKey === info.contextKey && prev.contextId === info.contextId && prev.tabLabel === info.tabLabel && prev.cwd === info.cwd) return;
  sessions.set(id, info);
  notify();
}

export function unregisterTermSession(id: string): void {
  if (sessions.delete(id)) notify();
}

export function termSessionInfo(id: string): TermSessionInfo | undefined {
  return sessions.get(id);
}

// termSessionCountForContext counts live terminal sessions owned by a context —
// the "you are about to lose running work" signal for close confirmations.
export function termSessionCountForContext(contextKey: string): number {
  let n = 0;
  for (const info of sessions.values()) {
    if (info.contextKey === contextKey) n++;
  }
  return n;
}

// termSessionCountForCwd counts live terminals rooted at one repo path — the
// per-repo badge on the editor's repo tiles.
export function termSessionCountForCwd(cwd: string): number {
  let n = 0;
  for (const info of sessions.values()) {
    if (info.cwd === cwd) n++;
  }
  return n;
}
