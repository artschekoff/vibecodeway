import { useState } from "react";
import { useEditorFiles, tabKey } from "../../contexts/EditorFilesContext";
import { TabContextMenu } from "../TabContextMenu";

export function EditorFileTabBar({ contextId }: { contextId: string }) {
  const { getFiles, closeEditorFile, setActiveEditorFile, pinEditorFile, closeAllEditorFiles } = useEditorFiles();
  const { tabs, activeKey } = getFiles(contextId);
  const [menu, setMenu] = useState<{ x: number; y: number; key: string; pinned: boolean } | null>(null);
  if (tabs.length === 0) return null;
  return (
    <div style={{ display: "flex", gap: 2, borderBottom: "1px solid var(--border)", overflowX: "auto" }}>
      {tabs.map((t) => {
        const key = tabKey(t);
        return (
          <div key={key} style={{ display: "flex", alignItems: "center", gap: 4, padding: "2px 8px",
            background: key === activeKey ? "var(--surface2)" : "transparent", fontSize: 11 }}
            onContextMenu={(e) => {
              e.preventDefault();
              setMenu({ x: e.clientX, y: e.clientY, key, pinned: !!t.pinned });
            }}>
            <button style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text)" }}
              onClick={() => setActiveEditorFile(contextId, key)}>{t.path.split("/").pop()}</button>
            <button aria-label={`close ${t.path}`} style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)" }}
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
          onCloseAll={() => closeAllEditorFiles(contextId)}
          onTogglePin={() => pinEditorFile(contextId, menu.key, !menu.pinned)}
          onDismiss={() => setMenu(null)}
        />
      )}
    </div>
  );
}
