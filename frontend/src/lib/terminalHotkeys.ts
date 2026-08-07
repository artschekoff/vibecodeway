// Residual terminal-panel keymap for the bindings the hotkey LAYER does not own.
// ⌘T (new / here / insert), ⌘⇧] / ⌘⇧[ (next / prev) now go through the rebindable
// hotkey layer (registry `terminal.*` + useHotkey). This keeps the cmux keys that
// have no layer command yet, matched here on the panel's capture listener:
//
//   ⌘W                 close active terminal
//   ⌃Tab               next terminal
//   ⌃⇧Tab              previous terminal
//   ⌃1–8               jump to terminal 1–8
//   ⌃9                 jump to last terminal

export type TerminalHotkey =
  | { kind: "close" }
  | { kind: "next" }
  | { kind: "prev" }
  | { kind: "jump"; index: number }; // 0-based; -1 = last

type KeyLike = Pick<KeyboardEvent, "metaKey" | "ctrlKey" | "altKey" | "shiftKey" | "key" | "code">;

export function matchTerminalHotkey(e: KeyLike): TerminalHotkey | null {
  // Exactly-Cmd / exactly-Ctrl (no other modifiers) so e.g. ⌃⌥1 isn't hijacked.
  const meta = e.metaKey && !e.ctrlKey && !e.altKey;
  const ctrl = e.ctrlKey && !e.metaKey && !e.altKey;

  if (meta && !e.shiftKey && e.key.toLowerCase() === "w") return { kind: "close" };

  if (ctrl && !e.shiftKey && e.code === "Tab") return { kind: "next" };
  if (ctrl && e.shiftKey && e.code === "Tab") return { kind: "prev" };

  if (ctrl && !e.shiftKey && /^[1-9]$/.test(e.key)) {
    const d = parseInt(e.key, 10);
    return d === 9 ? { kind: "jump", index: -1 } : { kind: "jump", index: d - 1 };
  }

  return null;
}
