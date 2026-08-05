import { useLayoutEffect, useRef, useState } from "react";

export type TabContextMenuProps = {
  x: number;
  y: number;
  isPinned: boolean;
  onClose: () => void;
  onCloseOthers: () => void;
  onTogglePin: () => void;
  onDismiss: () => void;
};

const itemStyle: React.CSSProperties = {
  display: "block", width: "100%", textAlign: "left", background: "none", border: "none",
  cursor: "pointer", color: "var(--text)", fontSize: 12, padding: "4px 10px", borderRadius: 4,
};

/** Cursor-positioned right-click menu shared by the sidebar and editor tab bars. */
export function TabContextMenu({ x, y, isPinned, onClose, onCloseOthers, onTogglePin, onDismiss }: TabContextMenuProps) {
  const run = (fn: () => void) => () => { fn(); onDismiss(); };

  // Clamp to the viewport so a right-click near an edge doesn't push the menu
  // off-screen. useLayoutEffect measures + repositions before paint (no flicker).
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ x, y });
  useLayoutEffect(() => {
    const el = menuRef.current;
    if (!el) return;
    const pad = 8;
    const { width, height } = el.getBoundingClientRect();
    setPos({
      x: Math.max(pad, Math.min(x, window.innerWidth - width - pad)),
      y: Math.max(pad, Math.min(y, window.innerHeight - height - pad)),
    });
  }, [x, y]);

  return (
    <>
      <div
        data-testid="tab-context-menu-backdrop"
        onClick={onDismiss}
        onContextMenu={(e) => { e.preventDefault(); onDismiss(); }}
        style={{ position: "fixed", inset: 0, zIndex: 1000 }}
      />
      <div
        role="menu"
        ref={menuRef}
        style={{
          position: "fixed", top: pos.y, left: pos.x, zIndex: 1001, minWidth: 140,
          background: "var(--bg-panel, var(--bg))", border: "1px solid var(--border)",
          borderRadius: 6, padding: 4, boxShadow: "0 6px 20px rgba(0,0,0,0.3)", color: "var(--text)",
        }}
      >
        <button role="menuitem" style={itemStyle} onClick={run(onClose)}>Close</button>
        <button role="menuitem" style={itemStyle} onClick={run(onCloseOthers)}>Close all but this</button>
        <button role="menuitem" style={itemStyle} onClick={run(onTogglePin)}>{isPinned ? "Unpin" : "Pin"}</button>
      </div>
    </>
  );
}
