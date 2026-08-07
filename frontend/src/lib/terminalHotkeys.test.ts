import { describe, it, expect } from "vitest";
import { matchTerminalHotkey, type TerminalHotkey } from "./terminalHotkeys";

const ev = (o: Partial<KeyboardEvent>): KeyboardEvent =>
  ({ metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, key: "", code: "", ...o } as KeyboardEvent);

describe("matchTerminalHotkey", () => {
  const cases: [string, Partial<KeyboardEvent>, TerminalHotkey | null][] = [
    ["Cmd+W → close", { metaKey: true, key: "w", code: "KeyW" }, { kind: "close" }],
    // ⌘T (new) and ⌘⇧] / ⌘⇧[ (next/prev) now belong to the hotkey layer, not here.
    ["Cmd+T → nothing (layer owns it)", { metaKey: true, key: "t", code: "KeyT" }, null],
    ["Cmd+Shift+] → nothing (layer owns it)", { metaKey: true, shiftKey: true, key: "}", code: "BracketRight" }, null],
    ["Ctrl+Tab → next", { ctrlKey: true, key: "Tab", code: "Tab" }, { kind: "next" }],
    ["Ctrl+Shift+Tab → prev", { ctrlKey: true, shiftKey: true, key: "Tab", code: "Tab" }, { kind: "prev" }],
    ["Ctrl+1 → jump 0", { ctrlKey: true, key: "1", code: "Digit1" }, { kind: "jump", index: 0 }],
    ["Ctrl+8 → jump 7", { ctrlKey: true, key: "8", code: "Digit8" }, { kind: "jump", index: 7 }],
    ["Ctrl+9 → jump last", { ctrlKey: true, key: "9", code: "Digit9" }, { kind: "jump", index: -1 }],
    // Negatives: must not hijack normal shell input.
    ["Ctrl+C → nothing", { ctrlKey: true, key: "c", code: "KeyC" }, null],
    ["plain t → nothing", { key: "t", code: "KeyT" }, null],
    ["Ctrl+Alt+1 → nothing (extra modifier)", { ctrlKey: true, altKey: true, key: "1", code: "Digit1" }, null],
    ["Cmd+Ctrl+T → nothing (extra modifier)", { metaKey: true, ctrlKey: true, key: "t", code: "KeyT" }, null],
  ];
  for (const [name, partial, expected] of cases) {
    it(name, () => expect(matchTerminalHotkey(ev(partial))).toEqual(expected));
  }
});
