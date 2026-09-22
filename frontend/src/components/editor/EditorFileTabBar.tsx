import { useState } from "react";
import { useEditorFiles, tabKey } from "../../contexts/EditorFilesContext";
import { TabContextMenu } from "../TabContextMenu";

// Middle-truncated filename ("denial-intelligence.sch…test.ts"): the start span
// shrinks with an ellipsis while the last few chars (extension) stay visible.
// CSS text-overflow can only ellipsize the end, hence the two-span split.
function TabName({ name }: { name: string }) {
  const tail = name.length > 8 ? name.slice(-7) : "";
  const head = tail ? name.slice(0, -7) : name;
  return (
    <span style={{ display: "flex", minWidth: 0, overflow: "hidden" }} title={name}>
      <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{head}</span>
      {tail && <span style={{ whiteSpace: "nowrap", flexShrink: 0 }}>{tail}</span>}
    </span>
  );
}

export function EditorFileTabBar({ contextId }: { contextId: string }) {
  const { getFiles, closeEditorFile, setActiveEditorFile, pinEditorFile, closeOtherEditorFiles } = useEditorFiles();
  const { tabs, activeKey } = getFiles(contextId);
  const [menu, setMenu] = useState<{ x: number; y: number; key: string; pinned: boolean } | null>(null);
  if (tabs.length === 0) return null;
  return (
    // userSelect none: the two-span middle-truncated label is plain text, so a
    // double-click used to select the tail span and paint it like a broken
    // highlight. Tabs are controls, not copyable text.
    <div style={{ display: "flex", gap: 2, borderBottom: "1px solid var(--border)", overflowX: "auto", userSelect: "none" }}>
      {tabs.map((t) => {
        const key = tabKey(t);
        return (
          <div key={key} style={{ display: "flex", alignItems: "center", gap: 4, padding: "2px 8px",
            width: 150, flexShrink: 0, boxSizing: "border-box", whiteSpace: "nowrap",
            background: key === activeKey ? "var(--surface2)" : "transparent", fontSize: 11 }}
            onContextMenu={(e) => {
              e.preventDefault();
              setMenu({ x: e.clientX, y: e.clientY, key, pinned: !!t.pinned });
            }}>
            <button style={{ flex: 1, minWidth: 0, display: "flex", overflow: "hidden", background: "none", border: "none", cursor: "pointer", color: "var(--text)", padding: 0, textAlign: "left" }}
              onClick={() => setActiveEditorFile(contextId, key)}>
              <TabName name={t.path.split("/").pop() ?? t.path} />
            </button>
            <button aria-label={`close ${t.path}`} style={{ flexShrink: 0, background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)" }}
              onClick={() => closeEditorFile(contextId, key)}>×</button>
          </div>
        );
      })}
      {menu && (
        <TabContextMenu
          x={menu.x}
          y={menu.y}
          isPinned={menu.pinned}
          onClose={() => closeEditorFile(contextId, menu.key)}
          onCloseOthers={() => closeOtherEditorFiles(contextId, menu.key)}
          onTogglePin={() => pinEditorFile(contextId, menu.key, !menu.pinned)}
          onDismiss={() => setMenu(null)}
        />
      )}
    </div>
  );
}
