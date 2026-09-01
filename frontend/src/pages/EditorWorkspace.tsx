import { useEffect, useMemo, useRef, useState } from "react";
import { RefreshCw, Copy, Check } from "lucide-react";
import { GetContextChanges, RefreshContext, RefreshContextPRs } from "../../wailsjs/go/main/App";
import { TerminalPanel } from "../components/editor/TerminalPanel";
import { ChangedFilesPanel } from "../components/editor/ChangedFilesPanel";
import { FileTreePanel } from "../components/editor/FileTreePanel";
import { DiffEditorTab } from "../components/editor/DiffEditorTab";
import { ResizeHandle } from "../components/editor/ResizeHandle";
import { EditorPRInfo } from "../components/editor/EditorPRInfo";
import { NotesPanel } from "../components/editor/NotesPanel";
import { nextPaneSize, readStoredPaneSize, fitPaneSize } from "../lib/paneSize";
import { useEditorFiles, tabKey, type OpenTab } from "../contexts/EditorFilesContext";
import { EditorSettingsProvider } from "../contexts/EditorSettingsContext";
import { EditorFocusProvider, useEditorFocus } from "../contexts/EditorFocusContext";
import { useHotkey } from "../hotkeys/HotkeyProvider";
import { useRovingTab } from "../hotkeys/useRovingTab";
import { editorModeForKey, editorModeLabel } from "../lib/contextMode";

const SIDEBAR_KEY = "vibecodeway:editor:sidebarWidth";
const SIDEBAR_MIN = 180;
const SIDEBAR_MAX = 640;
const SIDEBAR_DEFAULT = 300;

const NOTES_HEIGHT = 260;

const TERMINAL_KEY = "vibecodeway:editor:terminalHeight";
const TERMINAL_MIN = 90;
const TERMINAL_MAX = 900;
const TERMINAL_DEFAULT = 260;
// Editor rows that must survive however far the terminal is dragged. Without
// a floor the editor collapses to zero, the terminal overflows its column,
// and the resize handle is pushed off the top where it cannot be grabbed.
const EDITOR_MIN = 120;

export function EditorWorkspace(props: { contextId: string; contextKey?: string; initialCwd?: string }) {
  return (
    <EditorSettingsProvider>
      <EditorFocusProvider>
        <EditorWorkspaceInner {...props} />
      </EditorFocusProvider>
    </EditorSettingsProvider>
  );
}

function EditorWorkspaceInner({ contextId, contextKey, initialCwd }: { contextId: string; contextKey?: string; initialCwd?: string }) {
  const { active: activeZone, register: registerZone } = useEditorFocus();
  // ponytail: roving Tab only on the sidebar (buttons + tree). The editor and
  // terminal zones host Monaco/xterm, which own Tab natively — hijacking it there
  // would break indent and terminal completion.
  const sidebarRef = useRef<HTMLElement | null>(null);
  useRovingTab(sidebarRef, { order: "ttb-rtl" });
  const [repoPaths, setRepoPaths] = useState<{ id: string; name: string; path: string }[]>([]);
  const [cwd, setCwd] = useState<string>("");
  const [refreshSignal, setRefreshSignal] = useState(0);
  const [sidebarTab, setSidebarTab] = useState<"files" | "git">("git");
  const [pendingTerminalInsert, setPendingTerminalInsert] = useState<{ text: string; id: number } | null>(null);
  const [pendingTerminalOpen, setPendingTerminalOpen] = useState<{ cwd: string; id: number } | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [prRefreshing, setPrRefreshing] = useState(false);
  const [prRefreshError, setPrRefreshError] = useState("");
  const [copied, setCopied] = useState(false);
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

  // The editor column's live height. TERMINAL_MAX alone is a fixed 900px that
  // knows nothing about the window, so a height stored on a big screen (or a
  // hard drag) used to exceed the column entirely.
  const columnRef = useRef<HTMLDivElement>(null);
  const [columnHeight, setColumnHeight] = useState(0);
  useEffect(() => {
    const el = columnRef.current;
    if (!el) return;
    const measure = () => setColumnHeight(el.clientHeight);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  // Persist what the user chose, render what fits: their size comes back
  // untouched when the window grows again.
  const visibleTerminalHeight = fitPaneSize(terminalHeight, columnHeight, EDITOR_MIN, TERMINAL_MIN);

  // Sidebar FILES/GIT via the ⌘B leader: ⌘B F -> FILES, ⌘B G -> GIT. Scope
  // "editor" so it's active only while the editor/sidebar zone is focused.
  useHotkey("sidebar.files", () => setSidebarTab("files"), { scope: "editor" });
  useHotkey("sidebar.git", () => setSidebarTab("git"), { scope: "editor" });

  useEffect(() => {
    let active = true;
    void GetContextChanges(contextId).then((repos) => {
      if (!active) return;
      const paths = (repos ?? [])
        .filter((r) => r.repoPath)
        .map((r) => ({ id: r.repoId, name: r.repoName, path: r.repoPath }));
      setRepoPaths(paths);
      if (paths.length > 0) {
        const preferred = initialCwd && paths.some((p) => p.path === initialCwd) ? initialCwd : paths[0].path;
        setCwd((prev) => prev || preferred);
      }
    });
    return () => { active = false; };
  }, [contextId, refreshSignal]);

  const openFile = (repoPath: string, repoName: string, path: string, status: string, baseMode: "branch" | "head" = "head") => {
    const tab: OpenTab = { repoPath, repoName, path, status, baseMode };
    openEditorFile(contextId, tab);
  };

  // Re-ask GitHub for the freshest PR/branch state, then bump refreshSignal so the
  // PR head, changed-files list and open diffs all re-fetch and re-render.
  const handleRefresh = async () => {
    if (refreshing) return;
    setRefreshing(true);
    try {
      await RefreshContext(contextId);
    } catch {
      /* fail-soft: still re-read local state below */
    } finally {
      setRefreshSignal((n) => n + 1);
      setRefreshing(false);
    }
  };

  // PR-only re-sync: pulls the freshest PRs for the context's branches from the
  // remote and re-renders EditorPRInfo. Surfaces the error inline (RefreshContext
  // swallows VCS failures), so a failed sync is visible instead of showing nothing.
  const handleRefreshPRs = async () => {
    if (prRefreshing) return;
    setPrRefreshing(true);
    setPrRefreshError("");
    try {
      await RefreshContextPRs(contextId);
      setRefreshSignal((n) => n + 1);
    } catch (e) {
      setPrRefreshError(e instanceof Error ? e.message : "Failed to refresh PRs");
    } finally {
      setPrRefreshing(false);
    }
  };

  const active = useMemo(() => tabs.find((t) => tabKey(t) === activeKey), [tabs, activeKey]);
  const activeRepo = useMemo(() => repoPaths.find((r) => r.path === cwd), [repoPaths, cwd]);
  const activeRepoId = activeRepo?.id;

  return (
    <div style={{ display: "flex", height: "100%", flex: 1, minWidth: 0, width: "100%" }}>
      <div ref={columnRef} style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <div ref={(el) => registerZone("editor", el)} tabIndex={-1}
          className={activeZone === "editor" ? "zone--active" : undefined}
          style={{ flex: 1, minHeight: 0 }}>
          {active ? (
            <DiffEditorTab key={activeKey} repoPath={active.repoPath} path={active.path} baseMode={active.baseMode} refreshSignal={refreshSignal}
              onSaved={() => { /* panel auto-polls */ }}
              onAskAi={(text) => setPendingTerminalInsert((prev) => ({ text, id: (prev?.id ?? 0) + 1 }))} />
          ) : (
            <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 16 }}>Select a changed file to view its diff.</div>
          )}
        </div>
        <ResizeHandle
          orientation="horizontal"
          ariaLabel="resize terminal"
          onResize={(dy) => setTerminalHeight(nextPaneSize(visibleTerminalHeight, -dy, TERMINAL_MIN, TERMINAL_MAX))}
        />
        <div ref={(el) => registerZone("terminal", el)} tabIndex={-1}
          className={activeZone === "terminal" ? "zone--active" : undefined}
          style={{ height: visibleTerminalHeight, flexShrink: 0, minHeight: 0, borderTop: "1px solid var(--border)" }}>
          {/* Mounted unconditionally: TerminalPanel owns the ⌘T hotkey handlers, so
              gating it on `cwd` made ⌘T a silent no-op until GetContextChanges
              resolved (and forever for a context with no repo on disk). The panel
              handles an empty spawnCwd itself and seeds its first terminal once
              a path arrives. */}
          <TerminalPanel spawnCwd={cwd} contextName={contextKey} pendingInsert={pendingTerminalInsert ?? undefined} pendingOpen={pendingTerminalOpen ?? undefined} />
        </div>
      </div>
      <ResizeHandle
        orientation="vertical"
        ariaLabel="resize sidebar"
        onResize={(dx) => setSidebarWidth((w) => nextPaneSize(w, -dx, SIDEBAR_MIN, SIDEBAR_MAX))}
      />
      <aside ref={(el) => { sidebarRef.current = el; registerZone("sidebar", el); }} tabIndex={-1}
        className={activeZone === "sidebar" ? "zone--active" : undefined}
        style={{ width: sidebarWidth, borderLeft: "1px solid var(--border)", display: "flex", flexDirection: "column", flexShrink: 0 }}>
        {contextKey && (
          <div style={{ padding: "10px 12px", borderBottom: "1px solid var(--border)" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6, minWidth: 0 }}>
              <div
                title={contextKey}
                style={{ fontSize: 12, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
              >
                {contextKey}
              </div>
              <button
                aria-label="copy context name"
                title={copied ? "Copied" : "Copy context name"}
                onClick={() => {
                  void navigator.clipboard?.writeText(contextKey);
                  setCopied(true);
                  window.setTimeout(() => setCopied(false), 1200);
                }}
                style={{ display: "flex", flexShrink: 0, background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 2 }}
              >
                {copied ? <Check size={12} /> : <Copy size={12} />}
              </button>
            </div>
            <div style={{ fontSize: 10, textTransform: "uppercase", letterSpacing: 0.3, color: "var(--text-muted)", marginTop: 2 }}>
              {editorModeLabel(editorModeForKey(contextKey))}
            </div>
          </div>
        )}
        {/* Scratch log pinned to the top of the sidebar: what you did here, for when
            you come back to this context. Height-bounded so it never crowds out the
            file tree — the note list scrolls inside. */}
        <div style={{ maxHeight: NOTES_HEIGHT, display: "flex", flexDirection: "column", minHeight: 0, borderBottom: "1px solid var(--border)" }}>
          <NotesPanel contextId={contextId} />
        </div>
        {/* Repo selector sits above the Files/Git tabs — it scopes both. */}
        {repoPaths.length > 1 && (
          <select value={cwd} onChange={(e) => setCwd(e.target.value)} style={{ fontSize: 11, margin: 8 }}>
            {repoPaths.map((r) => <option key={r.path} value={r.path}>{r.name}</option>)}
          </select>
        )}
        {/* Files (full repo tree) vs Git (changes + PRs), like VSCode's activity bar. */}
        <div style={{ display: "flex", borderBottom: "1px solid var(--border)" }}>
          {(["files", "git"] as const).map((t) => (
            <button key={t} onClick={() => setSidebarTab(t)} aria-label={`${t} tab`}
              style={{ flex: 1, padding: "6px 8px", fontSize: 10, textTransform: "uppercase", letterSpacing: 0.4,
                background: sidebarTab === t ? "var(--surface2)" : "transparent",
                color: sidebarTab === t ? "var(--text)" : "var(--text-muted)",
                border: "none", borderBottom: sidebarTab === t ? "2px solid var(--accent, #7aa2f7)" : "2px solid transparent",
                cursor: "pointer" }}>
              {t === "files" ? "Files" : "Git"}
            </button>
          ))}
        </div>

        {sidebarTab === "files" ? (
          cwd ? (
            <FileTreePanel repoPath={cwd} repoName={activeRepo?.name ?? ""} onOpenFile={openFile}
              onOpenTerminal={(repoPath) => setPendingTerminalOpen((prev) => ({ cwd: repoPath, id: (prev?.id ?? 0) + 1 }))} />
          ) : (
            <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No repo on disk.</div>
          )
        ) : (
          <>
            {/* Always-visible PR header so a just-created PR can be pulled even when
                none are cached yet (EditorPRInfo renders nothing at zero PRs). */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "6px 12px", borderBottom: "1px solid var(--border)" }}>
              <span style={{ fontSize: 10, textTransform: "uppercase", letterSpacing: 0.3, color: "var(--text-muted)" }}>Pull requests</span>
              <button
                onClick={() => void handleRefreshPRs()}
                disabled={prRefreshing}
                title="Refresh PRs from GitHub"
                aria-label="refresh PRs"
                style={{ display: "flex", alignItems: "center", gap: 4, background: "none", border: "none", cursor: prRefreshing ? "default" : "pointer", color: "var(--text-muted)", fontSize: 11, padding: 2 }}
              >
                <RefreshCw size={12} className={prRefreshing ? "spinning" : undefined} />
              </button>
            </div>
            {prRefreshError && (
              <div style={{ fontSize: 11, color: "var(--danger, #e5534b)", padding: "4px 12px" }}>{prRefreshError}</div>
            )}
            <EditorPRInfo contextId={contextId} activeRepoId={repoPaths.length > 1 ? activeRepoId : undefined} refreshSignal={refreshSignal} />
            <ChangedFilesPanel contextId={contextId} onOpenFile={openFile}
              onOpenTerminal={(repoPath) => setPendingTerminalOpen((prev) => ({ cwd: repoPath, id: (prev?.id ?? 0) + 1 }))}
              onRefresh={() => void handleRefresh()} refreshing={refreshing}
              activeRepoPath={cwd} refreshSignal={refreshSignal} />
          </>
        )}
      </aside>
    </div>
  );
}
