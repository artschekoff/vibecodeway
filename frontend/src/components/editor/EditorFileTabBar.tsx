import { useEditorFiles, tabKey } from "../../contexts/EditorFilesContext";

export function EditorFileTabBar({ contextId }: { contextId: string }) {
  const { getFiles, closeEditorFile, setActiveEditorFile } = useEditorFiles();
  const { tabs, activeKey } = getFiles(contextId);
  if (tabs.length === 0) return null;
  return (
    <div style={{ display: "flex", gap: 2, borderBottom: "1px solid var(--border)", overflowX: "auto" }}>
      {tabs.map((t) => {
        const key = tabKey(t);
        return (
          <div key={key} style={{ display: "flex", alignItems: "center", gap: 4, padding: "2px 8px",
            background: key === activeKey ? "var(--surface2)" : "transparent", fontSize: 11 }}>
            <button style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text)" }}
              onClick={() => setActiveEditorFile(contextId, key)}>{t.path.split("/").pop()}</button>
            <button aria-label={`close ${t.path}`} style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)" }}
              onClick={() => closeEditorFile(contextId, key)}>×</button>
          </div>
        );
      })}
    </div>
  );
}
