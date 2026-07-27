import { useEffect, useMemo, useState } from "react";
import { GetContextChanges } from "../../wailsjs/go/main/App";
import { Terminal } from "../components/editor/Terminal";
import { ChangedFilesPanel } from "../components/editor/ChangedFilesPanel";
import { DiffEditorTab } from "../components/editor/DiffEditorTab";
import { ResizeHandle } from "../components/editor/ResizeHandle";
import { nextPaneSize, readStoredPaneSize } from "../lib/paneSize";
import { useEditorFiles, tabKey, type OpenTab } from "../contexts/EditorFilesContext";

const SIDEBAR_KEY = "context-consolidator:editor:sidebarWidth";
const SIDEBAR_MIN = 180;
const SIDEBAR_MAX = 640;
const SIDEBAR_DEFAULT = 300;

const TERMINAL_KEY = "context-consolidator:editor:terminalHeight";
const TERMINAL_MIN = 90;
const TERMINAL_MAX = 900;
const TERMINAL_DEFAULT = 260;

export function EditorWorkspace({ contextId }: { contextId: string }) {
  const [repoPaths, setRepoPaths] = useState<{ name: string; path: string }[]>([]);
  const [cwd, setCwd] = useState<string>("");
  const { getFiles, openEditorFile } = useEditorFiles();
  const { tabs, activeKey } = getFiles(contextId);
  const [sidebarWidth, setSidebarWidth] = useState(() =>
    readStoredPaneSize(localStorage.getItem(SIDEBAR_KEY), SIDEBAR_DEFAULT, SIDEBAR_MIN, SIDEBAR_MAX)
  );
  const [terminalHeight, setTerminalHeight] = useState(() =>
    readStoredPaneSize(localStorage.getItem(TERMINAL_KEY), TERMINAL_DEFAULT, TERMINAL_MIN, TERMINAL_MAX)
  );
  useEffect(() => { localStorage.setItem(SIDEBAR_KEY, String(sidebarWidth)); }, [sidebarWidth]);
  useEffect(() => { localStorage.setItem(TERMINAL_KEY, String(terminalHeight)); }, [terminalHeight]);

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
    openEditorFile(contextId, tab);
  };

  const active = useMemo(() => tabs.find((t) => tabKey(t) === activeKey), [tabs, activeKey]);

  return (
    <div style={{ display: "flex", height: "100%" }}>
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <div style={{ flex: 1, minHeight: 0 }}>
          {active ? (
            <DiffEditorTab key={activeKey} repoPath={active.repoPath} path={active.path} onSaved={() => { /* panel auto-polls */ }} />
          ) : (
            <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 16 }}>Select a changed file to view its diff.</div>
          )}
        </div>
        <ResizeHandle
          orientation="horizontal"
          ariaLabel="resize terminal"
          onResize={(dy) => setTerminalHeight((h) => nextPaneSize(h, -dy, TERMINAL_MIN, TERMINAL_MAX))}
        />
        <div style={{ height: terminalHeight, borderTop: "1px solid var(--border)" }}>
          {cwd ? <Terminal key={cwd} cwd={cwd} /> : <div style={{ fontSize: 12, padding: 12 }}>No repo path on disk.</div>}
        </div>
      </div>
      <ResizeHandle
        orientation="vertical"
        ariaLabel="resize sidebar"
        onResize={(dx) => setSidebarWidth((w) => nextPaneSize(w, -dx, SIDEBAR_MIN, SIDEBAR_MAX))}
      />
      <aside style={{ width: sidebarWidth, borderLeft: "1px solid var(--border)", display: "flex", flexDirection: "column", flexShrink: 0 }}>
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
