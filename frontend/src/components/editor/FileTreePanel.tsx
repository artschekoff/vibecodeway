import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronsDownUp, ChevronsUpDown, Terminal, Folder, File } from "lucide-react";
import { GetRepoFileTree } from "../../../wailsjs/go/main/App";
import { buildFileTree, type FileTreeNode } from "../../lib/fileTree";
import { useHotkey, useHotkeyScope } from "../../hotkeys/HotkeyProvider";
import { useTabVisible } from "../../contexts/TabVisibleContext";

type Props = {
  repoPath: string;
  repoName: string;
  // Same signature as the changed-files panel; status is "" for a plain file.
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string, baseMode: "branch" | "head") => void;
  // Open a new in-app terminal at this repo's local path.
  onOpenTerminal?: (repoPath: string) => void;
};

// fontFamily/fontSize must be set explicitly on <button>s — they don't inherit
// the surrounding font, which is why file names looked a different size.
const FONT: React.CSSProperties = { fontSize: 11, fontFamily: "inherit", lineHeight: "18px" };
const INDENT_PX = 14; // per tree depth
const rowStyle: React.CSSProperties = { display: "flex", alignItems: "center", gap: 4, padding: "1px 0", color: "var(--text)", ...FONT };
const dirRowStyle: React.CSSProperties = {
  display: "flex", alignItems: "center", gap: 4, width: "100%", background: "none", border: "none",
  cursor: "pointer", color: "var(--text)", padding: "1px 0", textAlign: "left", ...FONT,
};
const nameBtnStyle: React.CSSProperties = {
  flex: 1, minWidth: 0, textAlign: "left", background: "none", border: "none", cursor: "pointer",
  color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", padding: 0, ...FONT,
};
const nameSpanStyle: React.CSSProperties = { flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" };
const gutter: React.CSSProperties = { width: 12, flexShrink: 0, display: "inline-flex", justifyContent: "center", fontSize: 9, color: "var(--text-muted)" };
const iconStyle: React.CSSProperties = { flexShrink: 0, color: "var(--text-muted)" };
const toggleAllBtn: React.CSSProperties = {
  display: "flex", alignItems: "center", background: "none", border: "1px solid var(--border)",
  borderRadius: 4, cursor: "pointer", color: "var(--text-muted)", padding: "3px 5px", flexShrink: 0,
};

function collectDirPaths(nodes: FileTreeNode[], acc: string[]): string[] {
  for (const n of nodes) if (n.type === "dir") { acc.push(n.path); collectDirPaths(n.children, acc); }
  return acc;
}

// Flat, in-order list of the currently-visible rows (respecting open folders),
// for arrow-key navigation.
type FlatRow = { path: string; type: "file" | "dir" };
function flattenVisible(nodes: FileTreeNode[], isOpen: (p: string) => boolean, acc: FlatRow[] = []): FlatRow[] {
  for (const n of nodes) {
    acc.push({ path: n.path, type: n.type });
    if (n.type === "dir" && isOpen(n.path)) flattenVisible(n.children, isOpen, acc);
  }
  return acc;
}

/** VSCode-style full file tree for one repo; folders collapsed by default. */
export function FileTreePanel({ repoPath, repoName, onOpenFile, onOpenTerminal }: Props) {
  const [paths, setPaths] = useState<string[]>([]);
  const [expanded, setExpanded] = useState<Set<string>>(new Set()); // empty = all collapsed
  const [filter, setFilter] = useState("");
  const [activePath, setActivePath] = useState<string | null>(null);
  const [reloadSignal, setReloadSignal] = useState(0);
  const [focused, setFocused] = useState(false);
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let active = true;
    setPaths([]);
    setExpanded(new Set()); // reset to collapsed when switching repos
    void GetRepoFileTree(repoPath)
      .then((p) => { if (active) setPaths(p ?? []); })
      .catch(() => { if (active) setPaths([]); });
    return () => { active = false; };
  }, [repoPath, reloadSignal]);

  const q = filter.trim().toLowerCase();
  const shown = useMemo(() => (q ? paths.filter((p) => p.toLowerCase().includes(q)) : paths), [paths, q]);
  const tree = useMemo(() => buildFileTree(shown), [shown]);
  const allDirs = useMemo(() => collectDirPaths(tree, []), [tree]);

  const toggle = (path: string) =>
    setExpanded((prev) => { const n = new Set(prev); n.has(path) ? n.delete(path) : n.add(path); return n; });
  const toggleAll = () => setExpanded(expanded.size ? new Set() : new Set(allDirs));

  // An active filter forces every folder open so matches are visible.
  const isOpen = (path: string) => q !== "" || expanded.has(path);

  // The FILES zone owns "files" scope only while it holds DOM focus — so a plain
  // ⌘F elsewhere still opens global search (⌘F here becomes the file-search leader).
  // Also gated on tab visibility: every open editor tab keeps its panel
  // mounted, and the dispatcher picks the first registered handler.
  const tabVisible = useTabVisible();
  useHotkeyScope("files", focused && tabVisible);
  useHotkey("files.reload", () => setReloadSignal((n) => n + 1), { scope: "files", enabled: tabVisible });
  useHotkey("files.focusSearch", () => searchRef.current?.focus(), { scope: "files", enabled: tabVisible });

  const nodeByPath = useMemo(() => {
    const m = new Map<string, FileTreeNode>();
    const walk = (ns: FileTreeNode[]) => ns.forEach((n) => { m.set(n.path, n); if (n.type === "dir") walk(n.children); });
    walk(tree);
    return m;
  }, [tree]);

  const openFile = (path: string) => onOpenFile(repoPath, repoName, path, "", "head");

  const onTreeKeyDown = (e: React.KeyboardEvent) => {
    const flat = flattenVisible(tree, isOpen);
    if (flat.length === 0) return;
    const idx = activePath ? flat.findIndex((r) => r.path === activePath) : -1;
    const node = activePath ? nodeByPath.get(activePath) : undefined;

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setActivePath(flat[idx < 0 ? 0 : Math.min(idx + 1, flat.length - 1)].path);
        return;
      case "ArrowUp":
        e.preventDefault();
        setActivePath(flat[Math.max(idx - 1, 0)].path);
        return;
      case "ArrowRight":
        if (!node) return;
        e.preventDefault();
        if (node.type === "file") { openFile(node.path); return; }
        if (!isOpen(node.path)) toggle(node.path);            // open the folder…
        if (node.children[0]) setActivePath(node.children[0].path); // …and step into its first child
        return;
      case "ArrowLeft": {
        if (!node) return;
        e.preventDefault();
        if (node.type === "dir" && isOpen(node.path)) { toggle(node.path); return; } // collapse in place
        const parent = node.path.includes("/") ? node.path.slice(0, node.path.lastIndexOf("/")) : "";
        if (parent && nodeByPath.has(parent)) { if (isOpen(parent)) toggle(parent); setActivePath(parent); }
        return;
      }
      case " ":
        if (!node) return;
        e.preventDefault();
        if (node.type === "file") openFile(node.path); else toggle(node.path);
        return;
    }
  };

  const renderNodes = (nodes: FileTreeNode[], depth: number): React.ReactNode[] =>
    nodes.flatMap((n) => {
      const indent = 4 + depth * INDENT_PX;
      const activeCls = n.path === activePath ? "file-row--active" : undefined;
      if (n.type === "file") {
        return [
          <div key={`f:${n.path}`} className={activeCls} style={{ ...rowStyle, paddingLeft: indent }}>
            <span style={gutter} />
            <File size={13} style={iconStyle} />
            <button style={nameBtnStyle} title={n.path} onClick={() => openFile(n.path)}>{n.name}</button>
          </div>,
        ];
      }
      const open = isOpen(n.path);
      return [
        <button key={`d:${n.path}`} className={activeCls} onClick={() => toggle(n.path)} style={{ ...dirRowStyle, paddingLeft: indent }}>
          <span style={gutter}>{open ? "▾" : "▸"}</span>
          <Folder size={13} style={iconStyle} />
          <span style={nameSpanStyle}>{n.name}/</span>
        </button>,
        ...(open ? renderNodes(n.children, depth + 1) : []),
      ];
    });

  return (
    <div
      tabIndex={0}
      onKeyDown={onTreeKeyDown}
      onFocus={() => setFocused(true)}
      onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFocused(false); }}
      // flex:1 + minHeight:0 — without them this flex child's min-height is its
      // content, so a tall tree squeezes the notes block above instead of scrolling.
      style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "8px 10px", outline: "none" }}
    >
      <div style={{ display: "flex", gap: 6, marginBottom: 8 }}>
        <input
          ref={searchRef}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter files…"
          aria-label="filter repo files"
          style={{ flex: 1, minWidth: 0, boxSizing: "border-box", fontSize: 11, padding: "3px 6px", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, color: "var(--text)" }}
        />
        <button onClick={() => onOpenTerminal?.(repoPath)} style={toggleAllBtn}
          aria-label={`open terminal in ${repoName}`} title="Open terminal here">
          <Terminal size={13} />
        </button>
        <button onClick={toggleAll} style={toggleAllBtn}
          aria-label={expanded.size ? "collapse all" : "expand all"}
          title={expanded.size ? "Collapse all" : "Expand all"}>
          {expanded.size ? <ChevronsDownUp size={13} /> : <ChevronsUpDown size={13} />}
        </button>
      </div>
      {paths.length === 0 ? (
        <div style={{ fontSize: 12, color: "var(--text-muted)" }}>No files.</div>
      ) : (
        renderNodes(tree, 0)
      )}
    </div>
  );
}
