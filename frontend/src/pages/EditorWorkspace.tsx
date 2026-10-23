import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { RefreshCw, Copy, Check, ExternalLink } from "lucide-react";
import { GetContextChanges, RefreshContext, RefreshContextPRs, StartCodeTour, GetCodeTour, SymbolsIndex, CancelRefresh } from "../../wailsjs/go/main/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { useProgress } from "../contexts/ProgressContext";
import type { CodeTour } from "../types";
import { TerminalPanel } from "../components/editor/TerminalPanel";
import { ChangedFilesPanel } from "../components/editor/ChangedFilesPanel";
import { FileTreePanel } from "../components/editor/FileTreePanel";
import { DiffEditorTab } from "../components/editor/DiffEditorTab";
import { TourCard } from "../components/editor/TourCard";
import { SymbolPalette } from "../components/editor/SymbolPalette";
import { ResizeHandle } from "../components/editor/ResizeHandle";
import { EditorPRInfo } from "../components/editor/EditorPRInfo";
import { NotesPanel } from "../components/editor/NotesPanel";
import { nextPaneSize, readStoredPaneSize, fitPaneSize } from "../lib/paneSize";
import { useEditorFiles, tabKey, type OpenTab } from "../contexts/EditorFilesContext";
import { registerMonacoSymbols, releaseMonacoSymbols } from "../lib/monacoSymbols";
import { EditorSettingsProvider } from "../contexts/EditorSettingsContext";
import { EditorFocusProvider, useEditorFocus } from "../contexts/EditorFocusContext";
import { useHotkey } from "../hotkeys/HotkeyProvider";
import { useTabVisible } from "../contexts/TabVisibleContext";
import { useTabContextOptional } from "../contexts/TabContext";
import { useRovingTab } from "../hotkeys/useRovingTab";
import { editorModeForKey, editorModeLabel } from "../lib/contextMode";
import { repoTileLabel } from "../lib/repoTileLabel";
import { subscribeTermSessions, termSessionsVersion, termSessionCountForCwd } from "../lib/terminalRegistry";

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
  const progress = useProgress();
  const { active: activeZone, register: registerZone } = useEditorFocus();
  // ponytail: roving Tab only on the sidebar (buttons + tree). The editor and
  // terminal zones host Monaco/xterm, which own Tab natively — hijacking it there
  // would break indent and terminal completion.
  const sidebarRef = useRef<HTMLElement | null>(null);
  useRovingTab(sidebarRef, { order: "ttb-rtl" });
  const [repoPaths, setRepoPaths] = useState<{ id: string; name: string; path: string; branch?: string }[]>([]);
  const [cwd, setCwd] = useState<string>("");
  const [refreshSignal, setRefreshSignal] = useState(0);
  const [sidebarTab, setSidebarTab] = useState<"files" | "git">("git");
  // Owned here (not in DiffEditorTab) so fullscreen survives switching between
  // open-file tabs — each switch remounts DiffEditorTab (key=activeKey).
  const [editorFullscreen, setEditorFullscreen] = useState(false);
  const [pendingTerminalInsert, setPendingTerminalInsert] = useState<{ text: string; id: number } | null>(null);
  const [pendingTerminalOpen, setPendingTerminalOpen] = useState<{ cwd: string; id: number } | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [prRefreshing, setPrRefreshing] = useState(false);
  const [prRefreshError, setPrRefreshError] = useState("");
  const [copied, setCopied] = useState(false);
  // repoPath -> number of changed files, from the same GetContextChanges fetch
  // that populates repoPaths — drives the Start-tour button's disabled state.
  const [repoFileCounts, setRepoFileCounts] = useState<Record<string, number>>({});
  const [changesError, setChangesError] = useState("");
  const [tour, setTour] = useState<CodeTour | null>(null);
  const [tourOpen, setTourOpen] = useState(false);
  const [tourStarting, setTourStarting] = useState(false);
  const [tourError, setTourError] = useState("");
  const [symbolsOpen, setSymbolsOpen] = useState(false);
  // What the quick-open palette starts with: "" files, "#" symbols, "*" everything.
  const [paletteSeed, setPaletteSeed] = useState("");
  const openPalette = (seed: string) => { setPaletteSeed(seed); setSymbolsOpen(true); };
  // Which (repoPath, path, anchor) the diff editor should highlight once that
  // file's DiffEditorTab is mounted — set by TourCard as the user steps through.
  const [tourAnchorTarget, setTourAnchorTarget] = useState<{ repoPath: string; path: string; anchor: string } | null>(null);
  const { getFiles, openEditorFile } = useEditorFiles();
  const tabCtx = useTabContextOptional();
  // Re-render when terminals start/exit so the repo tiles' counters stay live.
  useSyncExternalStore(subscribeTermSessions, termSessionsVersion);
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
  const tabVisible = useTabVisible();
  // Re-measured whenever the tab becomes visible: a hidden tab (display:none)
  // measures 0 and any layout change that happens while hidden leaves a stale
  // height behind — the panes then render off the old number until something
  // (e.g. a window resize) pokes the observers. tabVisible in the deps forces
  // a fresh measure on every show.
  useEffect(() => {
    if (!tabVisible) return;
    const el = columnRef.current;
    if (!el) return;
    const measure = () => setColumnHeight(el.clientHeight);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [tabVisible]);
  // Persist what the user chose, render what fits: their size comes back
  // untouched when the window grows again.
  const visibleTerminalHeight = fitPaneSize(terminalHeight, columnHeight, EDITOR_MIN, TERMINAL_MIN);

  // Sidebar FILES/GIT via the ⌘B leader: ⌘B F -> FILES, ⌘B G -> GIT. Scope
  // "editor" so it's active only while the editor/sidebar zone is focused.
  // Gated on tab visibility: hidden editor tabs keep registering too, and the
  // dispatcher would otherwise pick the first-mounted (possibly hidden) one.
  useHotkey("sidebar.files", () => setSidebarTab("files"), { scope: "editor", enabled: tabVisible });
  useHotkey("sidebar.git", () => setSidebarTab("git"), { scope: "editor", enabled: tabVisible });

  useEffect(() => {
    let active = true;
    setChangesError("");
    void GetContextChanges(contextId).then((repos) => {
      if (!active) return;
      const paths = (repos ?? [])
        .filter((r) => r.repoPath)
        .map((r) => ({ id: r.repoId, name: r.repoName, path: r.repoPath, branch: r.branch }));
      setRepoPaths(paths);
      const counts: Record<string, number> = {};
      (repos ?? []).forEach((r) => { counts[r.repoPath] = (counts[r.repoPath] ?? 0) + (r.files?.length ?? 0); });
      setRepoFileCounts(counts);
      if (paths.length > 0) {
        const preferred = initialCwd && paths.some((p) => p.path === initialCwd) ? initialCwd : paths[0].path;
        setCwd((prev) => prev || preferred);
      }
    }).catch((e) => {
      // A transient failure here (e.g. DB contention at startup) used to brick
      // the whole editor silently — no repos, no terminal, no explanation.
      if (active) setChangesError(e instanceof Error ? e.message : String(e));
    });
    return () => { active = false; };
  }, [contextId, refreshSignal]);

  // Detect an already-running tour for this repo (e.g. after a tab switch or
  // app restart) so the sidebar can offer "Resume tour" instead of nothing.
  useEffect(() => {
    if (!cwd) { setTour(null); return; }
    let active = true;
    void GetCodeTour(contextId, cwd)
      .then((t) => { if (active) setTour(t ?? null); })
      .catch(() => { if (active) setTour(null); });
    return () => { active = false; };
  }, [contextId, cwd]);

  // Warm the symbol index as soon as the workspace knows its repo, so the
  // first go-to-definition doesn't pay the ctags build. Fire-and-forget:
  // ctags-not-found here is silent (the palette shows the install hint).
  useEffect(() => { if (cwd) void SymbolsIndex(cwd).catch(() => {}); }, [cwd]);

  const openFile = (repoPath: string, repoName: string, path: string, status: string, baseMode: "branch" | "head" = "head", revealLine?: number) => {
    const tab: OpenTab = { repoPath, repoName, path, status, baseMode, revealLine };
    openEditorFile(contextId, tab);
  };

  // Re-ask GitHub for the freshest PR/branch state, then bump refreshSignal so the
  // PR head, changed-files list and open diffs all re-fetch and re-render.
  const handleRefresh = async () => {
    if (refreshing) return;
    setRefreshing(true);
    // Same progress modal + per-provider steps as Context Detail's refresh —
    // RefreshContext can run for a minute on big contexts, and a bare spinning
    // icon reads as "hung".
    progress.begin("Reloading Workspace", [], () => { void CancelRefresh(); });
    const REFRESH_LABELS: Record<string, string> = {
      jira: "Syncing Jira issues & comments",
      slack: "Refreshing Slack threads & DMs",
      vcs: "Fetching pull requests",
      text: "Syncing texts & files",
    };
    const unsubscribe = EventsOn(
      "refresh:progress",
      (data: { current: number; total: number; step: string }) => {
        const label = REFRESH_LABELS[data.step] ?? `Syncing ${data.step}`;
        progress.set(data.current - 1, data.total, label);
      }
    );
    try {
      // Worktrees are never created here: a branch attached after the editor
      // opened is checked out only when the user picks it (Open Editor).
      await RefreshContext(contextId);
    } catch {
      /* fail-soft: still re-read local state below */
    } finally {
      unsubscribe();
      EventsOff("refresh:progress");
      progress.done();
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

  // StartCodeTour returns the stable "no changes to tour" error (spec §5) when
  // the repo's working tree has nothing to import/generate from — surfaced as
  // a quiet inline note instead of the generic error affordance.
  const handleStartTour = async () => {
    if (!cwd || tourStarting) return;
    setTourStarting(true);
    setTourError("");
    try {
      const t = await StartCodeTour(contextId, cwd);
      setTour(t);
      setTourOpen(true);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      setTourError(msg.includes("no changes to tour") ? "No changes to tour" : msg);
    } finally {
      setTourStarting(false);
    }
  };

  // ⌘⇧R — refresh this context; ⌘⇧U — start (or resume) the tour. Gated on
  // tab visibility like every per-tab handler (hidden tabs stay mounted).
  useHotkey("context.refresh", () => void handleRefresh(), { scope: "global", enabled: tabVisible });
  useHotkey("tour.start", () => {
    if (tour && !tourOpen) setTourOpen(true);
    else void handleStartTour();
  }, { scope: "editor", enabled: tabVisible });
  useHotkey("symbols.search", () => openPalette("#"), { scope: "editor", enabled: tabVisible });
  useHotkey("files.quickOpen", () => openPalette(""), { scope: "editor", enabled: tabVisible });
  useHotkey("search.code", () => openPalette("*"), { scope: "editor", enabled: tabVisible });

  const active = useMemo(() => tabs.find((t) => tabKey(t) === activeKey), [tabs, activeKey]);
  const activeRepo = useMemo(() => repoPaths.find((r) => r.path === cwd), [repoPaths, cwd]);
  const activeRepoId = activeRepo?.id;

  // registerMonacoSymbols registers monaco's providers/opener once (module-level
  // guard) but reactivates its "active deps" on every call — these refs let
  // that call always read the currently active tab's repo/context.
  const cwdRef = useRef(cwd);
  cwdRef.current = cwd;
  const contextIdRef = useRef(contextId);
  contextIdRef.current = contextId;
  const repoNameRef = useRef(activeRepo?.name ?? "");
  repoNameRef.current = activeRepo?.name ?? "";
  const repoPathsRef = useRef<string[]>([]);
  repoPathsRef.current = repoPaths.map((r) => r.path);
  // Hidden editor tabs stay mounted (see tabVisible comments above), so
  // re-run this whenever the tab becomes visible: the visible workspace must
  // always be the one that owns Monaco's shared symbol-nav deps, otherwise
  // F12/references in the active tab could read a hidden tab's cwd/contextId.
  useEffect(() => {
    if (!tabVisible) return;
    const deps = registerMonacoSymbols({
      getRepoPath: () => cwdRef.current,
      getRepoPaths: () => repoPathsRef.current,
      getContextId: () => contextIdRef.current,
      getRepoName: () => repoNameRef.current,
      openFile: openEditorFile,
    });
    // On unmount (tab closed), release the shared slot so this workspace's
    // closures are not retained; a newer workspace's deps are left alone.
    return () => releaseMonacoSymbols(deps);
  }, [tabVisible, cwd, contextId, activeRepo?.name, repoPaths, openEditorFile]);
  const hasChangesForTour = (repoFileCounts[cwd] ?? 0) > 0;
  const highlightAnchor =
    active && tourAnchorTarget && active.repoPath === tourAnchorTarget.repoPath && active.path === tourAnchorTarget.path
      ? tourAnchorTarget.anchor
      : undefined;

  return (
    <div style={{ display: "flex", height: "100%", flex: 1, minWidth: 0, width: "100%" }}>
      <div ref={columnRef} style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        <div ref={(el) => registerZone("editor", el)} tabIndex={-1}
          className={activeZone === "editor" ? "zone--active" : undefined}
          style={{ position: "relative", flex: 1, minHeight: 0, borderRadius: "var(--radius-sm)", overflow: "hidden", border: "1px solid var(--border)", margin: "6px 0 0 6px" }}>
          {active ? (
            <DiffEditorTab key={activeKey} repoPath={active.repoPath} path={active.path} baseMode={active.baseMode} refreshSignal={refreshSignal}
              highlightAnchor={highlightAnchor} revealLine={active.revealLine} revealNonce={active.revealNonce}
              fullscreen={editorFullscreen} onFullscreenChange={setEditorFullscreen}
              onSaved={() => { /* panel auto-polls */ }}
              onAskAi={(text) => setPendingTerminalInsert((prev) => ({ text, id: (prev?.id ?? 0) + 1 }))} />
          ) : (
            <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 16 }}>Select a changed file to view its diff.</div>
          )}
          {/* Docked bottom-right, inside the editor zone only — never covers the
              whole diff. Position persists via SetCodeTourStep on every step
              change (TourCard); closing here just hides it, GetCodeTour restores
              on the next "Resume tour". */}
          {tourOpen && tour && (
            <TourCard
              // Defense in depth alongside TourCard's own captured-tour-id
              // guard on AskTourQuestion: remounts the card outright when the
              // active tour's identity changes (new tour started, or the repo
              // selector swaps to a different repo's tour) instead of reusing
              // the instance with a swapped `tour` prop.
              key={tour.id}
              tour={tour}
              repoName={activeRepo?.name ?? ""}
              onClose={() => { setTourOpen(false); setTourAnchorTarget(null); }}
              onOpenFile={openFile}
              onLocateAnchor={(repoPath, path, anchor) => setTourAnchorTarget({ repoPath, path, anchor })}
              onAskAuthorAgent={(text) => setPendingTerminalInsert((prev) => ({ text, id: (prev?.id ?? 0) + 1 }))}
            />
          )}
          <SymbolPalette repoPath={cwd} open={symbolsOpen} initialQuery={paletteSeed} onClose={() => setSymbolsOpen(false)}
            onPick={(path, line) => openFile(cwd, activeRepo?.name ?? "", path, "", "branch", line)} />
        </div>
        <ResizeHandle
          orientation="horizontal"
          ariaLabel="resize terminal"
          // Functional update, because ResizeHandle registers its mousemove
          // listener once on mousedown and emits INCREMENTAL deltas: a height
          // captured at render time stays frozen for the whole drag, so only
          // the last step would ever apply. Clamping `h` first means a drag
          // that starts from an over-tall stored value moves from where the
          // handle actually is. columnHeight is closed over, but it cannot
          // change mid-drag — the window is not being resized at the same time.
          onResize={(dy) =>
            setTerminalHeight((h) =>
              nextPaneSize(fitPaneSize(h, columnHeight, EDITOR_MIN, TERMINAL_MIN), -dy, TERMINAL_MIN, TERMINAL_MAX))}
        />
        <div ref={(el) => registerZone("terminal", el)} tabIndex={-1}
          className={activeZone === "terminal" ? "zone--active" : undefined}
          style={{ height: visibleTerminalHeight, flexShrink: 0, minHeight: 0, border: "1px solid var(--border)", borderRadius: "var(--radius-sm)", overflow: "hidden", margin: "0 0 6px 6px" }}>
          {/* Mounted unconditionally: TerminalPanel owns the ⌘T hotkey handlers, so
              gating it on `cwd` made ⌘T a silent no-op until GetContextChanges
              resolved (and forever for a context with no repo on disk). The panel
              handles an empty spawnCwd itself and seeds its first terminal once
              a path arrives. */}
          <TerminalPanel spawnCwd={cwd} contextName={contextKey} persistKey={contextId} pendingInsert={pendingTerminalInsert ?? undefined} pendingOpen={pendingTerminalOpen ?? undefined}
            // A terminal tab bound to another repo became active → follow it in
            // the repo selector (only for repos this context actually links).
            onActiveCwd={(c) => setCwd((prev) => (repoPaths.some((r) => r.path === c) ? c : prev))} />
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
              <button
                title={`Open context ${contextKey}`}
                aria-label={`open context ${contextKey}`}
                // openContextTab focuses the context's tab when it is already
                // open, and opens a new one otherwise.
                onClick={() => tabCtx?.openContextTab(contextId, contextKey)}
                style={{ fontSize: 12, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", background: "none", border: "none", padding: 0, cursor: "pointer", color: "var(--text)", textAlign: "left" }}
              >
                {contextKey}
              </button>
              <button
                aria-label={`open context tab ${contextKey}`}
                title="Open context in its tab"
                onClick={() => tabCtx?.openContextTab(contextId, contextKey)}
                style={{ display: "flex", flexShrink: 0, background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: 2 }}
              >
                <ExternalLink size={12} />
              </button>
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
        {/* overflow:hidden is load-bearing twice over. maxHeight caps this box but
            not the paint, so without it tall notes render straight over the repo
            selector and the Files/Git tabs below. It also gives NotesPanel's inner
            overflowY:auto list a bounded height — flex:1 against an auto-height
            parent resolves to content size, so the list could never scroll. */}
        <div style={{ maxHeight: NOTES_HEIGHT, overflow: "hidden", display: "flex", flexDirection: "column", flexShrink: 0, borderBottom: "1px solid var(--border)" }}>
          <NotesPanel contextId={contextId} />
        </div>
        {/* Repo selector sits above the Files/Git tabs — it scopes both. */}
        {changesError && (
          <div role="alert" style={{ padding: "8px 12px", fontSize: 11, color: "var(--danger, #e5534b)", borderBottom: "1px solid var(--border)" }}>
            Failed to load the context&apos;s repos: {changesError}{" "}
            <button className="btn-secondary" style={{ fontSize: 11, padding: "1px 8px", marginLeft: 4 }}
              onClick={() => setRefreshSignal((n) => n + 1)}>
              Retry
            </button>
          </div>
        )}
        {!changesError && repoPaths.length === 0 && (
          <div style={{ padding: "8px 12px", fontSize: 11, color: "var(--text-muted)", borderBottom: "1px solid var(--border)" }}>
            No repos resolved on disk.{" "}
            <button className="btn-secondary" style={{ fontSize: 11, padding: "1px 8px", marginLeft: 4 }}
              onClick={() => setRefreshSignal((n) => n + 1)}>
              Retry
            </button>
          </div>
        )}
        {repoPaths.length > 1 && (
          // One-click repo switching: a wrapping tile grid instead of a select.
          // Fixed min column width + ellipsis inside — tiles never stretch or
          // clip however many worktrees (12+) the context links.
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(86px, 1fr))", gap: 4, padding: 8 }}>
            {repoPaths.map((r) => {
              const { top, sub } = repoTileLabel(r.name, r.branch);
              const selected = r.path === cwd;
              const terms = termSessionCountForCwd(r.path);
              return (
                <button
                  key={r.path}
                  onClick={() => setCwd(r.path)}
                  title={`${r.name}${r.branch ? ` · ${r.branch}` : ""}${terms > 0 ? ` · ${terms} terminal${terms === 1 ? "" : "s"}` : ""}`}
                  aria-pressed={selected}
                  style={{
                    position: "relative",
                    display: "flex", flexDirection: "column", alignItems: "flex-start", gap: 1,
                    padding: "4px 7px", minWidth: 0, overflow: "hidden", textAlign: "left",
                    borderRadius: 5, cursor: "pointer", font: "inherit",
                    background: selected ? "var(--surface2, rgba(127,127,127,0.15))" : "transparent",
                    border: selected ? "1px solid var(--accent, #e3b341)" : "1px solid var(--border)",
                    color: selected ? "var(--text)" : "var(--text-muted)",
                  }}
                >
                  <span style={{ fontSize: 11, fontWeight: 600, maxWidth: terms > 0 ? "calc(100% - 18px)" : "100%", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{top}</span>
                  {sub && <span style={{ fontSize: 10, maxWidth: "100%", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{sub}</span>}
                  {terms > 0 && (
                    <span aria-label={`${terms} running terminals`} style={{
                      position: "absolute", top: 3, right: 4,
                      fontSize: 9, lineHeight: "14px", minWidth: 14, textAlign: "center",
                      borderRadius: 7, padding: "0 3px",
                      background: "var(--accent, #e3b341)", color: "#1a1a1a", fontWeight: 700,
                    }}>{terms}</span>
                  )}
                </button>
              );
            })}
          </div>
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
          <button
            aria-label="reload editor workspace"
            title="Reload workspace: re-sync the context, pick up newly attached branches (⌘⇧R)"
            onClick={() => void handleRefresh()}
            disabled={refreshing}
            style={{ display: "flex", alignItems: "center", padding: "0 8px", background: "transparent",
              border: "none", borderBottom: "2px solid transparent", cursor: "pointer", color: "var(--text-muted)" }}
          >
            <RefreshCw size={11} className={refreshing ? "spinning" : undefined} />
          </button>
        </div>

        {sidebarTab === "git" && (
          <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "6px 12px", borderBottom: "1px solid var(--border)" }}>
            <button
              className="btn-secondary"
              style={{ fontSize: 11, padding: "2px 8px" }}
              disabled={!hasChangesForTour || tourStarting}
              title={!hasChangesForTour ? "No changes to tour" : "Start a guided tour of this repo's changes"}
              onClick={() => void handleStartTour()}
            >
              {tourStarting ? "Starting…" : "Start tour"}
            </button>
            {tour && !tourOpen && (
              <button
                aria-label="resume tour"
                title="Resume the active tour"
                onClick={() => setTourOpen(true)}
                style={{ fontSize: 11, background: "none", border: "none", color: "var(--accent, #7aa2f7)", cursor: "pointer", padding: 0 }}
              >
                Resume tour
              </button>
            )}
            {tourError && (
              <span style={{ fontSize: 11, color: tourError === "No changes to tour" ? "var(--text-muted)" : "var(--danger, #e5534b)", marginLeft: "auto" }}>
                {tourError}
              </span>
            )}
          </div>
        )}

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
            <EditorPRInfo contextId={contextId} activeRepoId={repoPaths.length > 1 ? activeRepoId : undefined}
              activeBranch={repoPaths.length > 1 ? activeRepo?.branch : undefined} refreshSignal={refreshSignal} />
            <ChangedFilesPanel contextId={contextId} onOpenFile={openFile}
              onOpenTerminal={(repoPath) => setPendingTerminalOpen((prev) => ({ cwd: repoPath, id: (prev?.id ?? 0) + 1 }))}
              activeRepoPath={cwd} refreshSignal={refreshSignal} />
          </>
        )}
      </aside>
    </div>
  );
}
