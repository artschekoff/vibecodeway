// One terminal tab as it survives the editor closing: where it ran, when it was
// opened, and the command that brings its coding-agent conversation back.
// `openedAt` is what stops an agent conversation from an earlier window being
// attributed to a tab that was opened after it.
export interface SavedTerminal {
  cwd: string;
  openedAt: number;
  resume?: string;
}

// Keyed by context id so each editor restores its own terminals. Same namespace
// as the other editor UI state (pane heights, sidebar widths).
const storageKey = (persistKey: string) => `vibecodeway:editor:terminals:${persistKey}`;

export function loadTerminals(persistKey: string): SavedTerminal[] {
  try {
    const raw = localStorage.getItem(storageKey(persistKey));
    const parsed: unknown = raw ? JSON.parse(raw) : null;
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((t): t is SavedTerminal => typeof t?.cwd === "string" && t.cwd.length > 0)
      .map((t) => ({ cwd: t.cwd, openedAt: typeof t.openedAt === "number" ? t.openedAt : 0, ...(typeof t.resume === "string" && t.resume ? { resume: t.resume } : {}) }));
  } catch {
    return []; // corrupt storage just means "no layout to restore"
  }
}

export function saveTerminals(persistKey: string, terminals: SavedTerminal[]): void {
  try {
    localStorage.setItem(storageKey(persistKey), JSON.stringify(terminals));
  } catch {
    /* quota or private mode — a lost layout is not worth failing the editor over */
  }
}
