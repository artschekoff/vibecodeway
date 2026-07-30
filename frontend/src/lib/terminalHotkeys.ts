// Terminal hotkeys mirroring cmux's "surface" (tab) keymap, scoped to the
// terminal panel. Split-pane / workspace bindings don't apply — our terminals
// are flat tabs. Reference: https://cmux.com/docs/keyboard-shortcuts
//
//   ⌘T                 new terminal
//   ⌘W                 close active terminal
//   ⌘⇧] / ⌃Tab         next terminal
//   ⌘⇧[ / ⌃⇧Tab        previous terminal
//   ⌃1–8               jump to terminal 1–8
//   ⌃9                 jump to last terminal
//
// Bracket keys are matched by e.code (BracketRight/Left) because Shift turns
// "]"/"[" into "}"/"{" in e.key.

export type TerminalHotkey =
  | { kind: "new" }
  | { kind: "close" }
  | { kind: "next" }
  | { kind: "prev" }
  | { kind: "jump"; index: number }; // 0-based; -1 = last

type KeyLike = Pick<KeyboardEvent, "metaKey" | "ctrlKey" | "altKey" | "shiftKey" | "key" | "code">;

export function matchTerminalHotkey(e: KeyLike): TerminalHotkey | null {
  // Exactly-Cmd / exactly-Ctrl (no other modifiers) so e.g. ⌃⌥1 isn't hijacked.
  const meta = e.metaKey && !e.ctrlKey && !e.altKey;
  const ctrl = e.ctrlKey && !e.metaKey && !e.altKey;

  if (meta && !e.shiftKey && e.key.toLowerCase() === "t") return { kind: "new" };
  if (meta && !e.shiftKey && e.key.toLowerCase() === "w") return { kind: "close" };

  if ((meta && e.shiftKey && e.code === "BracketRight") || (ctrl && !e.shiftKey && e.code === "Tab"))
    return { kind: "next" };
  if ((meta && e.shiftKey && e.code === "BracketLeft") || (ctrl && e.shiftKey && e.code === "Tab"))
    return { kind: "prev" };

  if (ctrl && !e.shiftKey && /^[1-9]$/.test(e.key)) {
    const d = parseInt(e.key, 10);
    return d === 9 ? { kind: "jump", index: -1 } : { kind: "jump", index: d - 1 };
  }

  return null;
}
