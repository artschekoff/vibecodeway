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
        style={{
          position: "fixed", top: y, left: x, zIndex: 1001, minWidth: 140,
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
