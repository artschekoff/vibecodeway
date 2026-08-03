import { useEffect, useMemo, useState } from "react";
import { ChevronsDownUp, ChevronsUpDown, Terminal, Folder, File } from "lucide-react";
import { GetRepoFileTree } from "../../../wailsjs/go/main/App";
import { buildFileTree, type FileTreeNode } from "../../lib/fileTree";

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

/** VSCode-style full file tree for one repo; folders collapsed by default. */
export function FileTreePanel({ repoPath, repoName, onOpenFile, onOpenTerminal }: Props) {
  const [paths, setPaths] = useState<string[]>([]);
  const [expanded, setExpanded] = useState<Set<string>>(new Set()); // empty = all collapsed
  const [filter, setFilter] = useState("");

  useEffect(() => {
    let active = true;
    setPaths([]);
    setExpanded(new Set()); // reset to collapsed when switching repos
    void GetRepoFileTree(repoPath)
      .then((p) => { if (active) setPaths(p ?? []); })
      .catch(() => { if (active) setPaths([]); });
    return () => { active = false; };
  }, [repoPath]);

  const q = filter.trim().toLowerCase();
  const shown = useMemo(() => (q ? paths.filter((p) => p.toLowerCase().includes(q)) : paths), [paths, q]);
  const tree = useMemo(() => buildFileTree(shown), [shown]);
  const allDirs = useMemo(() => collectDirPaths(tree, []), [tree]);

  const toggle = (path: string) =>
    setExpanded((prev) => { const n = new Set(prev); n.has(path) ? n.delete(path) : n.add(path); return n; });
  const toggleAll = () => setExpanded(expanded.size ? new Set() : new Set(allDirs));

  // An active filter forces every folder open so matches are visible.
  const isOpen = (path: string) => q !== "" || expanded.has(path);

  const renderNodes = (nodes: FileTreeNode[], depth: number): React.ReactNode[] =>
    nodes.flatMap((n) => {
      const indent = 4 + depth * INDENT_PX;
      if (n.type === "file") {
        return [
          <div key={`f:${n.path}`} style={{ ...rowStyle, paddingLeft: indent }}>
            <span style={gutter} />
            <File size={13} style={iconStyle} />
            <button style={nameBtnStyle} title={n.path} onClick={() => onOpenFile(repoPath, repoName, n.path, "", "head")}>{n.name}</button>
          </div>,
        ];
      }
      const open = isOpen(n.path);
      return [
        <button key={`d:${n.path}`} onClick={() => toggle(n.path)} style={{ ...dirRowStyle, paddingLeft: indent }}>
          <span style={gutter}>{open ? "▾" : "▸"}</span>
          <Folder size={13} style={iconStyle} />
          <span style={nameSpanStyle}>{n.name}/</span>
        </button>,
        ...(open ? renderNodes(n.children, depth + 1) : []),
      ];
    });

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      <div style={{ display: "flex", gap: 6, marginBottom: 8 }}>
        <input
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
