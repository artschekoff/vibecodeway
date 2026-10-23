import { useCallback, useEffect, useRef, useState } from "react";
import { Copy, Terminal, ChevronsDownUp, ChevronsUpDown, Folder, File as FileIcon, ExternalLink } from "lucide-react";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { GetContextChanges, StageFile, UnstageFile, DiscardFile, ConfirmAction, ResolveConflict } from "../../../wailsjs/go/main/App";
import { useTabVisible } from "../../contexts/TabVisibleContext";
import { main } from "../../../wailsjs/go/models";

type Props = {
  contextId: string;
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string, baseMode: "branch" | "head") => void;
  // Open a new in-app terminal at the repo's local path.
  onOpenTerminal?: (repoPath: string) => void;
  // Re-sync the context from GitHub (icon shown in the repo header).
  // When set, show only the repo whose repoPath matches (follows the editor's
  // repo selector). Unset => show all repos.
  activeRepoPath?: string;
  // Bumped by the sidebar Refresh button to force an immediate re-fetch (the panel
  // also polls on its own interval).
  refreshSignal?: number;
};

const POLL_MS = 2000;

// Compact ~22px rows: minHeight + tight padding + box-sizing so a button's own
// UA padding/border can't inflate the row past the target height.
const ROW_HEIGHT = 22;
// Per-level indent for nested folder/file rows, VS Code-style.
const INDENT_PX = 12;
const rowStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 6, width: "100%", minWidth: 0, boxSizing: "border-box",
  fontSize: 11, padding: "1px 4px", minHeight: ROW_HEIGHT, color: "var(--text)",
};
const nameBtnStyle: React.CSSProperties = {
  flex: 1, minWidth: 0, textAlign: "left", background: "none", border: "none", cursor: "pointer", padding: 0,
  color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
  fontSize: 11, fontFamily: "inherit", // buttons don't inherit font — keep names at row size
};
const actionBtnStyle: React.CSSProperties = {
  background: "none", border: "none", cursor: "pointer", color: "var(--text-muted)", padding: "0 2px",
};
const headerRowStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 2, marginBottom: 2,
};
const headerBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 4, flex: 1, minWidth: 0, background: "none", border: "none",
  cursor: "pointer", padding: 0, margin: 0, textAlign: "left",
  fontSize: 10, textTransform: "uppercase", color: "var(--text-muted)",
};
// Compact icon buttons for the per-section "collapse all" / "expand all" folder controls.
const treeToggleBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "none", cursor: "pointer",
  color: "var(--text-muted)", padding: "0 2px", flexShrink: 0,
};
const chevronStyle: React.CSSProperties = { width: 10, display: "inline-block", fontSize: 9, flexShrink: 0 };
const folderBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 6, width: "100%", minWidth: 0, boxSizing: "border-box",
  background: "none", border: "none", cursor: "pointer", textAlign: "left",
  fontSize: 11, padding: "1px 4px", minHeight: ROW_HEIGHT, color: "var(--text)",
};
const folderNameStyle: React.CSSProperties = {
  flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
};

// --- Folder-tree grouping ------------------------------------------------
// Groups a section's flat file list into a folder tree, compacting single-child
// folder chains VS Code-style (e.g. "src/functions/cf-metric" collapses onto one
// row when every level in between has exactly one child).
type TreeFile<T> = T & { path: string };
type TreeNode<T> =
  | { type: "file"; name: string; path: string; file: TreeFile<T> }
  | { type: "folder"; name: string; path: string; children: TreeNode<T>[] };
type FolderNode<T> = Extract<TreeNode<T>, { type: "folder" }>;

function buildFileTree<T extends { path: string }>(files: T[]): TreeNode<T>[] {
  type Building = { name: string; path: string; folders: Map<string, Building>; file?: T };
  const root: Building = { name: "", path: "", folders: new Map() };
  for (const f of files) {
    const parts = f.path.split("/").filter(Boolean);
    let cur = root;
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const path = parts.slice(0, i + 1).join("/");
      let next = cur.folders.get(part);
      if (!next) {
        next = { name: part, path, folders: new Map() };
        cur.folders.set(part, next);
      }
      if (i === parts.length - 1) next.file = f;
      cur = next;
    }
  }
  const toNodes = (b: Building): TreeNode<T>[] => {
    const entries = [...b.folders.values()];
    entries.sort((a, bb) => {
      const aIsFile = !!a.file, bIsFile = !!bb.file;
      if (aIsFile !== bIsFile) return aIsFile ? 1 : -1; // folders before files
      return a.name.localeCompare(bb.name);
    });
    return entries.map((e): TreeNode<T> => {
      if (e.file) return { type: "file", name: e.name, path: e.path, file: e.file };
      let node: FolderNode<T> = { type: "folder", name: e.name, path: e.path, children: toNodes(e) };
      // Compact chains of single-child folders into one row.
      while (node.children.length === 1 && node.children[0].type === "folder") {
        const child = node.children[0] as FolderNode<T>;
        node = { type: "folder", name: `${node.name}/${child.name}`, path: child.path, children: child.children };
      }
      return node;
    });
  };
  return toNodes(root);
}

function collectFolderPaths<T>(nodes: TreeNode<T>[]): string[] {
  const out: string[] = [];
  for (const n of nodes) {
    if (n.type === "folder") {
      out.push(n.path);
      out.push(...collectFolderPaths(n.children));
    }
  }
  return out;
}

// `sectionKey` namespaces folder collapse-state keys (`${sectionKey}::${folderPath}`)
// so the same folder path in two different sections/repos collapses independently.
function TreeRows<T>({ nodes, depth, sectionKey, collapsed, onToggleFolder, renderFile }: {
  nodes: TreeNode<T>[];
  depth: number;
  sectionKey: string;
  collapsed: Set<string>;
  onToggleFolder: (key: string) => void;
  renderFile: (file: TreeFile<T>) => React.ReactNode;
}) {
  return (
    <>
      {nodes.map((n) => {
        const indent = depth * INDENT_PX;
        if (n.type === "folder") {
          const fullKey = `${sectionKey}::${n.path}`;
          const isCollapsed = collapsed.has(fullKey);
          return (
            <div key={`d:${n.path}`}>
              <button
                style={{ ...folderBtnStyle, paddingLeft: 4 + indent }}
                onClick={() => onToggleFolder(fullKey)}
                aria-label={`${isCollapsed ? "expand" : "collapse"} folder ${n.path}`}
                title={n.path}
              >
                <span style={chevronStyle}>{isCollapsed ? "▸" : "▾"}</span>
                <Folder size={12} style={{ flexShrink: 0, color: "var(--text-muted)" }} />
                <span style={folderNameStyle}>{n.name}</span>
              </button>
              {!isCollapsed && (
                <TreeRows nodes={n.children} depth={depth + 1} sectionKey={sectionKey} collapsed={collapsed} onToggleFolder={onToggleFolder} renderFile={renderFile} />
              )}
            </div>
          );
        }
        return (
          <div key={`f:${n.path}`} style={{ ...rowStyle, paddingLeft: 4 + indent }}>
            <FileIcon size={12} style={{ flexShrink: 0, color: "var(--text-muted)" }} />
            {renderFile(n.file)}
          </div>
        );
      })}
    </>
  );
}

// Sections are collapsed by default; this stores which ones the user opened.
const EXPAND_KEY = "vibecodeway:changedfiles:expanded";

function loadExpanded(): Set<string> {
  try {
    const raw = localStorage.getItem(EXPAND_KEY);
    return new Set(raw ? (JSON.parse(raw) as string[]) : []);
  } catch {
    return new Set();
  }
}

// Shared empty set — used in place of an active collapse set while a filter is
// on, so every folder renders expanded (avoids allocating a new Set per render).
const EMPTY_SET: Set<string> = new Set();

const toggleAllBtnStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "1px solid var(--border)",
  borderRadius: 4, cursor: "pointer", color: "var(--text-muted)", padding: "3px 5px", flexShrink: 0,
};

export function ChangedFilesPanel({ contextId, onOpenFile, onOpenTerminal, activeRepoPath, refreshSignal }: Props) {
  const [repos, setRepos] = useState<main.RepoChange[]>([]);
  const [expanded, setExpanded] = useState<Set<string>>(() => loadExpanded());
  const [filter, setFilter] = useState("");
  // Folder-tree collapse state, keyed by `${sectionKey}::${folderPath}`. Component-local
  // (no persistence needed) and separate from `expanded`, which shows/hides whole sections.
  // Folders default to expanded.
  const [collapsedFolders, setCollapsedFolders] = useState<Set<string>>(new Set());
  const mountedRef = useRef(true);

  const persistExpanded = (next: Set<string>) => {
    try { localStorage.setItem(EXPAND_KEY, JSON.stringify([...next])); } catch { /* ignore */ }
  };
  const toggleSection = (key: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      persistExpanded(next);
      return next;
    });
  };

  const toggleFolder = (key: string) => {
    setCollapsedFolders((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  const collapseAllFolders = (keys: string[]) => {
    setCollapsedFolders((prev) => new Set([...prev, ...keys]));
  };
  const expandAllFolders = (keys: string[]) => {
    setCollapsedFolders((prev) => {
      const next = new Set(prev);
      keys.forEach((k) => next.delete(k));
      return next;
    });
  };

  // Single-flight (VS Code git-extension Throttler style): a slow
  // GetContextChanges must not pile up with the 2s poll — overlapping calls
  // resolved out of order and the sections flapped between snapshots.
  const inFlightRef = useRef(false);
  const load = useCallback(async () => {
    if (inFlightRef.current) return;
    inFlightRef.current = true;
    try {
      const r = await GetContextChanges(contextId);
      if (mountedRef.current) setRepos(r ?? []);
    } catch {
      /* transient; keep last known */
    } finally {
      inFlightRef.current = false;
    }
  }, [contextId]);

  // Poll ONLY while this editor tab is visible: hidden tabs stay mounted
  // (display:none keeps their terminals alive), and each poll spawns git
  // diff/status per repo — N open editor tabs used to multiply into dozens of
  // git forks per second, pinning the CPU. document.hidden also pauses the
  // visible tab's poll while the whole app is in the background.
  const tabVisible = useTabVisible();
  useEffect(() => {
    let active = true;
    mountedRef.current = true; // re-arm on every (re)run — deps now include refreshSignal
    if (!tabVisible) return () => { active = false; mountedRef.current = false; };
    const tick = async () => { if (active && !document.hidden) await load(); };
    void tick();
    const t = setInterval(tick, POLL_MS);
    return () => { active = false; mountedRef.current = false; clearInterval(t); };
  }, [load, refreshSignal, tabVisible]);

  const act = async (fn: (repoPath: string, path: string) => Promise<void>, repoPath: string, path: string) => {
    try {
      await fn(repoPath, path);
    } finally {
      await load(); // reflect the change immediately, don't wait for the poll
    }
  };

  const onDiscard = async (repoPath: string, path: string) => {
    // Native dialog — window.confirm() is a no-op in the Wails webview.
    if (await ConfirmAction("Discard changes", `Discard changes to ${path}? This cannot be undone.`)) {
      void act(DiscardFile, repoPath, path);
    }
  };

  const statusLetter = (s: string) => (s ? s.charAt(0).toUpperCase() : "");
  // Git status letter → colour: added/untracked green, deleted red, else (M/R/C) orange.
  const statusColor = (s: string) => {
    switch (statusLetter(s)) {
      case "A": case "U": case "?": return "#3fb950"; // green
      case "D": return "#f85149"; // red
      default: return "#d29922"; // orange (modified/renamed/etc.)
    }
  };

  const shown = activeRepoPath ? repos.filter((r) => r.repoPath === activeRepoPath) : repos;

  if (shown.length === 0) {
    return <div style={{ fontSize: 12, color: "var(--text-muted)", padding: 12 }}>No changes</div>;
  }

  const q = filter.trim().toLowerCase();
  const matchFile = (p: string) => !q || p.toLowerCase().includes(q);
  // A section is open when explicitly expanded, or while a filter is active
  // (so matches are visible even though sections default to collapsed).
  const sectionOpen = (key: string) => q !== "" || expanded.has(key);
  // While filtering, force every folder open too so matches stay visible under
  // their folders instead of being hidden by a stale collapse.
  const treeCollapsed = q !== "" ? EMPTY_SET : collapsedFolders;

  // Section header for a folder-tree section: the existing show/hide-section
  // toggle, plus (when the tree has folders) compact collapse-all/expand-all
  // icon buttons that operate on this section's folder-tree state only.
  const renderTreeHeader = (key: string, label: string, folderKeys: string[]) => (
    <div style={headerRowStyle}>
      <button style={headerBtnStyle} onClick={() => toggleSection(key)}>
        <span style={chevronStyle}>{sectionOpen(key) ? "▾" : "▸"}</span>{label}
      </button>
      {folderKeys.length > 0 && (
        <>
          <button aria-label={`collapse all folders in ${label}`} title="Collapse All" style={treeToggleBtnStyle}
            onClick={() => collapseAllFolders(folderKeys)}>
            <ChevronsDownUp size={11} />
          </button>
          <button aria-label={`expand all folders in ${label}`} title="Expand All" style={treeToggleBtnStyle}
            onClick={() => expandAllFolders(folderKeys)}>
            <ChevronsUpDown size={11} />
          </button>
        </>
      )}
    </div>
  );

  // Keys of every section that currently has (matching) content — for expand-all.
  const allSectionKeys: string[] = [];
  for (const repo of shown) {
    const files = (repo.files ?? []).filter((f) => matchFile(f.path));
    if (files.some((f) => f.commitStatus)) allSectionKeys.push(`${repo.repoName}:branch`);
    if (files.some((f) => f.indexStatus)) allSectionKeys.push(`${repo.repoName}:staged`);
    if (files.some((f) => f.workStatus)) allSectionKeys.push(`${repo.repoName}:local`);
  }
  const toggleAll = () => {
    const next = expanded.size ? new Set<string>() : new Set(allSectionKeys);
    setExpanded(next);
    persistExpanded(next);
  };

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      <div style={{ display: "flex", gap: 6, marginBottom: 8 }}>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter files…"
          aria-label="filter changed files"
          style={{ flex: 1, minWidth: 0, boxSizing: "border-box", fontSize: 11, padding: "3px 6px", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, color: "var(--text)" }}
        />
        <button onClick={toggleAll} style={toggleAllBtnStyle}
          aria-label={expanded.size ? "collapse all" : "expand all"}
          title={expanded.size ? "Collapse all" : "Expand all"}>
          {expanded.size ? <ChevronsDownUp size={13} /> : <ChevronsUpDown size={13} />}
        </button>
      </div>
      {shown.map((repo) => {
        const files = (repo.files ?? []).filter((f) => matchFile(f.path));
        // Branch (PR) changes vs the base branch — the full committed set, shown
        // on top like GitHub. Read-only: nothing to stage/discard.
        const branch = files.filter((f) => f.commitStatus);
        // Local uncommitted changes (independent of the branch list — a file in
        // the PR that's also edited locally appears in both).
        // Files git left unmerged. They carry no index/work status precisely so
        // they land here alone instead of being double-listed below.
        const conflicts = files.filter((f) => f.conflicted);
        const staged = files.filter((f) => f.indexStatus);
        const unstaged = files.filter((f) => f.workStatus);
        // Keep the repo header (and its refresh/copy/terminal actions) visible even
        // with no changes — otherwise the refresh button would vanish exactly when
        // you'd want to re-check for changes.
        const bKey = `${repo.repoName}:branch`;
        const sKey = `${repo.repoName}:staged`;
        const lKey = `${repo.repoName}:local`;
        const cKey = `${repo.repoName}:conflicts`;

        const conflictsTree = buildFileTree(conflicts);
        const branchTree = buildFileTree(branch);
        const stagedTree = buildFileTree(staged);
        const unstagedTree = buildFileTree(unstaged);
        const conflictsFolderKeys = collectFolderPaths(conflictsTree).map((p) => `${cKey}::${p}`);
        const branchFolderKeys = collectFolderPaths(branchTree).map((p) => `${bKey}::${p}`);
        const stagedFolderKeys = collectFolderPaths(stagedTree).map((p) => `${sKey}::${p}`);
        const unstagedFolderKeys = collectFolderPaths(unstagedTree).map((p) => `${lKey}::${p}`);

        return (
          <div key={repo.repoName} style={{ marginBottom: 12 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 4 }}>
              <span style={{ flex: 1, fontSize: 12, fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={repo.repoPath}>{repo.repoName}</span>
              {repo.branch && (
                <button aria-label={`open branch ${repo.branch} on GitHub`} title={`Open ${repo.branch} on GitHub`} style={actionBtnStyle}
                  onClick={() => BrowserOpenURL(`https://github.com/${repo.repoName}/tree/${repo.branch}`)}><ExternalLink size={12} /></button>
              )}
              <button aria-label={`copy path for ${repo.repoName}`} title="Copy repo path" style={actionBtnStyle} onClick={() => void navigator.clipboard?.writeText(repo.repoPath)}><Copy size={12} /></button>
              {/* No per-repo refresh button: local changes auto-poll while the
                  tab is visible, and the sidebar's reload button (⌘⇧R) does
                  the full re-sync — a third refresh here was duplicate noise. */}
              <button aria-label={`open terminal in ${repo.repoName}`} title="Open terminal here" style={actionBtnStyle} onClick={() => onOpenTerminal?.(repo.repoPath)}><Terminal size={12} /></button>
            </div>

            {repo.didHitLimit && (
              /* VS Code's git.statusLimit warning, adapted: the list is truncated, say so. */
              <div role="alert" style={{ fontSize: 11, color: "#d29922", margin: "0 0 6px", lineHeight: 1.4 }}>
                Too many active changes — only the first {repo.statusLimit.toLocaleString()} are shown.
                Add noise to .gitignore, or raise “Git status limit” in editor settings (0 = unlimited).
              </div>
            )}

            {conflicts.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                {renderTreeHeader(cKey, "Merge Conflicts", conflictsFolderKeys)}
                {sectionOpen(cKey) && (
                  <TreeRows
                    nodes={conflictsTree}
                    depth={0}
                    sectionKey={cKey}
                    collapsed={treeCollapsed}
                    onToggleFolder={toggleFolder}
                    renderFile={(f) => (
                      <>
                        <span style={{ width: 14, color: "#f85149" }} title="Unmerged">!</span>
                        {/* Opens the working-tree file with its conflict markers, so the
                            finer-grained resolution is editing it and marking it resolved. */}
                        <button style={nameBtnStyle} title={f.path} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, "conflict", "head")}>{f.path.split("/").pop()}</button>
                        <button aria-label={`keep ours for ${f.path}`} title="Keep ours (this branch)" style={actionBtnStyle}
                          onClick={() => void act((rp, p) => ResolveConflict(rp, p, "ours"), repo.repoPath, f.path)}>ours</button>
                        <button aria-label={`keep theirs for ${f.path}`} title="Keep theirs (branch being merged in)" style={actionBtnStyle}
                          onClick={() => void act((rp, p) => ResolveConflict(rp, p, "theirs"), repo.repoPath, f.path)}>theirs</button>
                        <button aria-label={`mark ${f.path} resolved`} title="Mark resolved (stage it)" style={actionBtnStyle}
                          onClick={() => void act(StageFile, repo.repoPath, f.path)}>✓</button>
                      </>
                    )}
                  />
                )}
              </div>
            )}

            {branch.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                {renderTreeHeader(bKey, "Branch Changes", branchFolderKeys)}
                {sectionOpen(bKey) && (
                  <TreeRows
                    nodes={branchTree}
                    depth={0}
                    sectionKey={bKey}
                    collapsed={treeCollapsed}
                    onToggleFolder={toggleFolder}
                    renderFile={(f) => (
                      <>
                        <span style={{ width: 14, color: statusColor(f.commitStatus) }}>{statusLetter(f.commitStatus)}</span>
                        <button style={nameBtnStyle} title={f.path} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.commitStatus, "branch")}>{f.path.split("/").pop()}</button>
                      </>
                    )}
                  />
                )}
              </div>
            )}

            {staged.length > 0 && (
              <div style={{ marginBottom: 6 }}>
                {renderTreeHeader(sKey, "Staged Changes", stagedFolderKeys)}
                {sectionOpen(sKey) && (
                  <TreeRows
                    nodes={stagedTree}
                    depth={0}
                    sectionKey={sKey}
                    collapsed={treeCollapsed}
                    onToggleFolder={toggleFolder}
                    renderFile={(f) => (
                      <>
                        <span style={{ width: 14, color: statusColor(f.indexStatus) }}>{statusLetter(f.indexStatus)}</span>
                        <button style={nameBtnStyle} title={f.path} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.indexStatus, "head")}>{f.path.split("/").pop()}</button>
                        <button aria-label={`unstage ${f.path}`} title="Unstage" style={actionBtnStyle} onClick={() => void act(UnstageFile, repo.repoPath, f.path)}>−</button>
                      </>
                    )}
                  />
                )}
              </div>
            )}

            {unstaged.length > 0 && (
              <div>
                {renderTreeHeader(lKey, "Local Changes", unstagedFolderKeys)}
                {sectionOpen(lKey) && (
                  <TreeRows
                    nodes={unstagedTree}
                    depth={0}
                    sectionKey={lKey}
                    collapsed={treeCollapsed}
                    onToggleFolder={toggleFolder}
                    renderFile={(f) => (
                      <>
                        <span style={{ width: 14, color: statusColor(f.workStatus) }}>{statusLetter(f.workStatus)}</span>
                        <button style={nameBtnStyle} title={f.path} onClick={() => onOpenFile(repo.repoPath, repo.repoName, f.path, f.workStatus, "head")}>{f.path.split("/").pop()}</button>
                        <button aria-label={`discard ${f.path}`} title="Discard" style={actionBtnStyle} onClick={() => void onDiscard(repo.repoPath, f.path)}>↺</button>
                        <button aria-label={`stage ${f.path}`} title="Stage" style={actionBtnStyle} onClick={() => void act(StageFile, repo.repoPath, f.path)}>+</button>
                      </>
                    )}
                  />
                )}
              </div>
            )}

            {files.length === 0 && (
              <div style={{ fontSize: 11, color: "var(--text-muted)", padding: "2px 4px" }}>
                {q ? "No matching files" : "No changes"}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
