import { useEffect, useMemo, useState } from "react";
import { GetContextChanges } from "../../wailsjs/go/main/App";
import { Terminal } from "../components/editor/Terminal";
import { ChangedFilesPanel } from "../components/editor/ChangedFilesPanel";
import { DiffEditorTab } from "../components/editor/DiffEditorTab";

type OpenTab = { repoPath: string; repoName: string; path: string; status: string };
const tabKey = (t: OpenTab) => `${t.repoPath}::${t.path}`;

export function EditorWorkspace({ contextId }: { contextId: string }) {
  const [repoPaths, setRepoPaths] = useState<{ name: string; path: string }[]>([]);
  const [cwd, setCwd] = useState<string>("");
  const [tabs, setTabs] = useState<OpenTab[]>([]);
  const [activeKey, setActiveKey] = useState<string>("");

  useEffect(() => {
    let active = true;
    void GetContextChanges(contextId).then((repos) => {
      if (!active) return;
      const paths = (repos ?? [])
        .filter((r) => r.repoPath)
        .map((r) => ({ name: r.repoName, path: r.repoPath }));
      setRepoPaths(paths);
      if (paths.length > 0) setCwd((prev) => prev || paths[0].path);
    });
    return () => { active = false; };
  }, [contextId]);

  const openFile = (repoPath: string, repoName: string, path: string, status: string) => {
    const tab: OpenTab = { repoPath, repoName, path, status };
    const key = tabKey(tab);
    setTabs((prev) => (prev.some((t) => tabKey(t) === key) ? prev : [...prev, tab]));
    setActiveKey(key);
  };
  const closeTab = (key: string) =>
    setTabs((prev) => {
      const next = prev.filter((t) => tabKey(t) !== key);
      if (activeKey === key) setActiveKey(next.length ? tabKey(next[next.length - 1]) : "");
      return next;
    });

  const active = useMemo(() => tabs.find((t) => tabKey(t) === activeKey), [tabs, activeKey]);

  return (
    <div style={{ display: "flex", height: "100%" }}>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        {/* editor tabs */}
        <div style={{ display: "flex", gap: 2, borderBottom: "1px solid var(--border)", overflowX: "auto" }}>
          {tabs.map((t) => {
            const key = tabKey(t);
            return (
              <div key={key} style={{ display: "flex", alignItems: "center", gap: 4, padding: "2px 8px",
                background: key === activeKey ? "var(--surface2)" : "transparent", fontSize: 11 }}>
                <button style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text)" }}
                  onClick={() => setActiveKey(key)}>{t.path.split("/").pop()}</button>
                <button aria-label={`close ${t.path}`} style={{ background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)" }}
                  onClick={() => closeTab(key)}>×</button>
              </div>
            );
          })}
        </div>
        <div style={{ flex: 1, minHeight: 0 }}>
          {active ? (
            <DiffEditorTab key={activeKey} repoPath={active.repoPath} path={active.path} onSaved={() => { /* panel auto-polls */ }} />
          ) : (
            <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 16 }}>Select a changed file to view its diff.</div>
          )}
        </div>
        <div style={{ height: "40%", borderTop: "1px solid var(--border)" }}>
          {cwd ? <Terminal key={cwd} cwd={cwd} /> : <div style={{ fontSize: 12, padding: 12 }}>No repo path on disk.</div>}
        </div>
      </div>
      <aside style={{ width: 300, borderLeft: "1px solid var(--border)", display: "flex", flexDirection: "column" }}>
        {repoPaths.length > 1 && (
          <select value={cwd} onChange={(e) => setCwd(e.target.value)} style={{ fontSize: 11, margin: 8 }}>
            {repoPaths.map((r) => <option key={r.path} value={r.path}>{r.name}</option>)}
          </select>
        )}
        <ChangedFilesPanel contextId={contextId} onOpenFile={openFile} />
      </aside>
    </div>
  );
}
