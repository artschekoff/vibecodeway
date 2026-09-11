export type Platform = "darwin" | "other";
export interface Chord { mod: boolean; ctrl: boolean; shift: boolean; alt: boolean; key: string }

const MODS_ONLY = new Set(["meta", "control", "shift", "alt"]);

/** Normalize a KeyboardEvent.key to a canonical lowercase token. */
function normKey(k: string): string {
  const low = k.toLowerCase();
  if (low === " ") return "space";
  return low; // "arrowup", "enter", "t", "[", "]", etc.
}

// Physical-key fallback for non-Latin layouts: on Russian layout ⌘I arrives as
// e.key="ш", so chords bound to letters never match. e.code names the physical
// key regardless of layout.
const CODE_KEYS: Record<string, string> = {
  BracketLeft: "[", BracketRight: "]", Semicolon: ";", Quote: "'",
  Comma: ",", Period: ".", Slash: "/", Backquote: "`",
};

function physicalKey(code: string): string | null {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3).toLowerCase();
  if (/^Digit[0-9]$/.test(code)) return code.slice(5);
  return CODE_KEYS[code] ?? null;
}

export function eventToToken(e: KeyboardEvent, platform: Platform): Chord | null {
  const raw = (e.key || "").toLowerCase();
  if (MODS_ONLY.has(raw)) return null;
  const modPressed = platform === "darwin" ? !!e.metaKey : !!e.ctrlKey;
  // On darwin, ctrlKey is a distinct raw modifier; on other, ctrl IS mod, so raw ctrl = false.
  const rawCtrl = platform === "darwin" ? !!e.ctrlKey : false;
  let key = normKey(e.key);
  // Only rewrite keys a Latin layout could never produce, so every existing
  // ASCII binding keeps today's behavior exactly.
  if (key.length === 1 && /[^\x20-\x7e]/.test(key)) {
    const phys = physicalKey(e.code || "");
    if (phys) key = phys;
  }
  return { mod: modPressed, ctrl: rawCtrl, shift: !!e.shiftKey, alt: !!e.altKey, key };
}

export function parseChord(s: string): Chord {
  const parts = s.toLowerCase().split("+");
  const key = parts[parts.length - 1];
  const has = (m: string) => parts.slice(0, -1).includes(m);
  return { mod: has("mod"), ctrl: has("ctrl"), shift: has("shift"), alt: has("alt"), key };
}

export function serializeChord(c: Chord): string {
  const p: string[] = [];
  if (c.mod) p.push("mod");
  if (c.ctrl) p.push("ctrl");
  if (c.shift) p.push("shift");
  if (c.alt) p.push("alt");
  p.push(c.key);
  return p.join("+");
}

const KEY_SYMBOLS: Record<string, string> = {
  arrowup: "↑", arrowdown: "↓", arrowleft: "←", arrowright: "→",
  enter: "↵", space: "␣", escape: "Esc",
};

export function formatChord(c: Chord, platform: Platform): string {
  const key = KEY_SYMBOLS[c.key] ?? c.key.toUpperCase();
  if (platform === "darwin") {
    return `${c.mod ? "⌘" : ""}${c.ctrl ? "⌃" : ""}${c.shift ? "⇧" : ""}${c.alt ? "⌥" : ""}${key}`;
  }
  const p: string[] = [];
  if (c.mod) p.push("Ctrl");
  if (c.ctrl) p.push("Ctrl");
  if (c.shift) p.push("Shift");
  if (c.alt) p.push("Alt");
  p.push(key);
  return p.join("+");
}
