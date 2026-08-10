import { createContext, useCallback, useContext, useRef, useState } from "react";
import { useHotkey, useHotkeyScope } from "../hotkeys/HotkeyProvider";

export type Zone = "editor" | "terminal" | "sidebar";
type Dir = "up" | "down" | "left" | "right";

// Ctrl+arrow adjacency. Layout: editor top-left, terminal bottom-left, sidebar right.
// ponytail: sidebar reaches editor (left) / terminal (down); editor↔terminal covers the rest.
const ADJ: Record<Zone, Partial<Record<Dir, Zone>>> = {
  editor: { down: "terminal", right: "sidebar" },
  terminal: { up: "editor", right: "sidebar" },
  sidebar: { left: "editor", down: "terminal" },
};

interface EditorFocusValue {
  active: Zone;
  setActive: (z: Zone) => void;
  register: (z: Zone, el: HTMLElement | null) => void;
}

const EditorFocusContext = createContext<EditorFocusValue | null>(null);

export function EditorFocusProvider({ children }: { children: React.ReactNode }) {
  const refs = useRef<Partial<Record<Zone, HTMLElement>>>({});
  const [active, setActiveState] = useState<Zone>("editor");
  const activeRef = useRef(active);
  activeRef.current = active;

  const setActive = useCallback((z: Zone) => {
    setActiveState(z);
    refs.current[z]?.focus();
  }, []);

  const register = useCallback((z: Zone, el: HTMLElement | null) => {
    if (el) refs.current[z] = el;
    else delete refs.current[z];
  }, []);

  const go = useCallback((dir: Dir) => {
    const next = ADJ[activeRef.current]?.[dir];
    if (next) setActive(next);
  }, [setActive]);

  // Editor zone owns the Ctrl+arrow scope while this provider (the editor tab) is mounted.
  useHotkeyScope("editor", true);
  useHotkey("editor.focusUp", () => go("up"), { scope: "editor" });
  useHotkey("editor.focusDown", () => go("down"), { scope: "editor" });
  useHotkey("editor.focusLeft", () => go("left"), { scope: "editor" });
  useHotkey("editor.focusRight", () => go("right"), { scope: "editor" });

  return (
    <EditorFocusContext.Provider value={{ active, setActive, register }}>
      {children}
    </EditorFocusContext.Provider>
  );
}

export function useEditorFocus(): EditorFocusValue {
  const ctx = useContext(EditorFocusContext);
  if (!ctx) throw new Error("useEditorFocus must be used within <EditorFocusProvider>");
  return ctx;
}
