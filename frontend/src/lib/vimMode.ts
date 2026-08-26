import { useEffect, useState } from "react";

const KEY = "vibecodeway:vimMode";
const EVENT = "vibecodeway:vimModeChanged";

// ponytail: localStorage, not a DB setting — this is a per-machine UI preference,
// same as the editor pane sizes. No migration, no Wails binding, no Go change.

export function readVimMode(): boolean {
  try {
    return localStorage.getItem(KEY) === "1";
  } catch {
    return false;
  }
}

export function setVimMode(on: boolean): void {
  try {
    localStorage.setItem(KEY, on ? "1" : "0");
  } catch {
    /* private mode / quota — the in-memory toggle below still fires */
  }
  window.dispatchEvent(new Event(EVENT));
}

/**
 * Reads the vim-mode preference and re-renders when it changes — including from
 * another editor mounted elsewhere in the app (custom event) or another window
 * (storage event). Every ProseEditor flips together.
 */
export function useVimMode(): boolean {
  const [on, setOn] = useState(readVimMode);
  useEffect(() => {
    const sync = () => setOn(readVimMode());
    window.addEventListener(EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);
  return on;
}
