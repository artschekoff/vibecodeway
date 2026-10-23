import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useHotkey, useHotkeyScope } from "../hotkeys/HotkeyProvider";
import { useTabVisible } from "./TabVisibleContext";

export type Zone = "editor" | "terminal" | "sidebar";
type Dir = "up" | "down" | "left" | "right";

// ⌘E + h/j/k/l (vim) adjacency. Layout: editor top-left, terminal bottom-left, sidebar right.
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
    const el = refs.current[z];
    if (!el) return;
    // Land the keyboard where typing actually goes: xterm and Monaco read
    // input from their own hidden textareas, so focusing the zone container
    // alone left ⌘N-switched zones deaf until a click. Visible inputs only
    // (hidden terminal tabs keep their textareas mounted); the LAST match wins
    // so a diff editor focuses its editable modified pane, not the read-only
    // original.
    const inputs = Array.from(
      el.querySelectorAll<HTMLElement>(".xterm-helper-textarea, textarea.inputarea"),
    ).filter((n) => n.offsetParent !== null);
    (inputs[inputs.length - 1] ?? el).focus();
  }, []);

  const register = useCallback((z: Zone, el: HTMLElement | null) => {
    if (el) refs.current[z] = el;
    else delete refs.current[z];
  }, []);

  const go = useCallback((dir: Dir) => {
    const next = ADJ[activeRef.current]?.[dir];
    if (next) setActive(next);
  }, [setActive]);

  // Editor zone owns the ⌘E leader scope while this editor tab is the VISIBLE
  // one. Hidden tabs stay mounted (display:none in TabContent), so keying this
  // on mere mount kept the "editor" scope active app-wide and let a hidden
  // tab's handlers win the dispatcher's first-registered pick.
  const tabVisible = useTabVisible();
  useHotkeyScope("editor", tabVisible);
  useHotkey("editor.focusUp", () => go("up"), { scope: "editor", enabled: tabVisible });
  useHotkey("editor.focusDown", () => go("down"), { scope: "editor", enabled: tabVisible });
  useHotkey("editor.focusLeft", () => go("left"), { scope: "editor", enabled: tabVisible });
  useHotkey("editor.focusRight", () => go("right"), { scope: "editor", enabled: tabVisible });

  // ── ⌘-hold zone navigation ──
  // Holding ⌘ overlays each zone with its number; ⌘1..⌘9 jumps focus there.
  // Inside a zone the arrows/Enter belong to that zone's own widgets (file
  // tree, xterm, Monaco, the sidebar's roving Tab).
  const [badges, setBadges] = useState<{ zone: Zone; n: number; rect: DOMRect }[] | null>(null);
  useEffect(() => {
    if (!tabVisible) { setBadges(null); return; }
    const showBadges = () => {
      const rows: { zone: Zone; n: number; rect: DOMRect }[] = [];
      ZONE_ORDER.forEach((z, i) => {
        const el = refs.current[z];
        if (!el) return;
        const rect = el.getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) rows.push({ zone: z, n: i + 1, rect });
      });
      setBadges(rows);
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Meta") { showBadges(); return; }
      if (e.metaKey && !e.ctrlKey && !e.altKey && e.key >= "1" && e.key <= "9") {
        const zone = ZONE_ORDER[Number(e.key) - 1];
        if (zone && refs.current[zone]) {
          e.preventDefault();
          e.stopPropagation();
          setActive(zone);
          setBadges(null);
        }
      }
    };
    const hideBadges = (e?: KeyboardEvent) => {
      if (!e || e.key === "Meta") setBadges(null);
    };
    const onBlur = () => setBadges(null);
    // Capture phase so xterm/Monaco (which swallow keydown) cannot eat ⌘N.
    window.addEventListener("keydown", onKeyDown, true);
    window.addEventListener("keyup", hideBadges, true);
    window.addEventListener("blur", onBlur);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
      window.removeEventListener("keyup", hideBadges, true);
      window.removeEventListener("blur", onBlur);
    };
  }, [tabVisible, setActive]);

  return (
    <EditorFocusContext.Provider value={{ active, setActive, register }}>
      {children}
      {badges && badges.length > 0 && createPortal(
        <>
          {badges.map((b) => (
            <div key={b.zone} aria-hidden style={{
              position: "fixed", left: b.rect.left + 10, top: b.rect.top + 10, zIndex: 300,
              width: 30, height: 30, borderRadius: 8,
              display: "flex", alignItems: "center", justifyContent: "center",
              fontSize: 16, fontWeight: 700,
              background: "var(--accent, #e3b341)", color: "#1a1a1a",
              boxShadow: "0 2px 10px rgba(0,0,0,0.45)", pointerEvents: "none",
            }}>{b.n}</div>
          ))}
        </>,
        document.body,
      )}
    </EditorFocusContext.Provider>
  );
}

// Zone numbering for ⌘-hold navigation: 1 editor, 2 terminal, 3 sidebar.
const ZONE_ORDER: Zone[] = ["editor", "terminal", "sidebar"];

export function useEditorFocus(): EditorFocusValue {
  const ctx = useContext(EditorFocusContext);
  if (!ctx) throw new Error("useEditorFocus must be used within <EditorFocusProvider>");
  return ctx;
}
