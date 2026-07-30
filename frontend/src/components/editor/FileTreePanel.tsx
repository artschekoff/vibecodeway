import { useEffect, useMemo, useState } from "react";
import { GetRepoFileTree } from "../../../wailsjs/go/main/App";
import { buildFileTree, type FileTreeNode } from "../../lib/fileTree";

type Props = {
  repoPath: string;
  repoName: string;
  // Same signature as the changed-files panel; status is "" for a plain file.
  onOpenFile: (repoPath: string, repoName: string, path: string, status: string) => void;
};

const rowStyle: React.CSSProperties = { display: "flex", alignItems: "center", fontSize: 11, padding: "1px 0" };
const nameBtnStyle: React.CSSProperties = {
  flex: 1, textAlign: "left", background: "none", border: "none", cursor: "pointer",
  color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", padding: "1px 0",
};
const chevron: React.CSSProperties = { width: 10, display: "inline-block", fontSize: 9, color: "var(--text-muted)" };

/** VSCode-style full file tree for one repo; clicking a file opens it in the editor. */
export function FileTreePanel({ repoPath, repoName, onOpenFile }: Props) {
  const [paths, setPaths] = useState<string[]>([]);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");

  useEffect(() => {
    let active = true;
    setPaths([]);
    void GetRepoFileTree(repoPath)
      .then((p) => { if (active) setPaths(p ?? []); })
      .catch(() => { if (active) setPaths([]); });
    return () => { active = false; };
  }, [repoPath]);

  const q = filter.trim().toLowerCase();
  const shown = useMemo(() => (q ? paths.filter((p) => p.toLowerCase().includes(q)) : paths), [paths, q]);
  const tree = useMemo(() => buildFileTree(shown), [shown]);

  const toggle = (key: string) =>
    setCollapsed((prev) => { const n = new Set(prev); n.has(key) ? n.delete(key) : n.add(key); return n; });

  const renderNodes = (nodes: FileTreeNode[], depth: number): React.ReactNode[] =>
    nodes.flatMap((n) => {
      const indent = 4 + depth * 12;
      if (n.type === "file") {
        return [
          <div key={`f:${n.path}`} style={{ ...rowStyle, paddingLeft: indent }}>
            <button style={nameBtnStyle} title={n.path} onClick={() => onOpenFile(repoPath, repoName, n.path, "")}>{n.name}</button>
          </div>,
        ];
      }
      const dirKey = `d:${n.path}`;
      const isCollapsed = collapsed.has(dirKey);
      return [
        <button key={dirKey} onClick={() => toggle(dirKey)}
          style={{ display: "flex", alignItems: "center", gap: 2, width: "100%", background: "none", border: "none",
            cursor: "pointer", color: "var(--text)", fontSize: 11, padding: "1px 0", paddingLeft: indent, textAlign: "left" }}>
          <span style={chevron}>{isCollapsed ? "▸" : "▾"}</span>{n.name}/
        </button>,
        ...(isCollapsed ? [] : renderNodes(n.children, depth + 1)),
      ];
    });

  return (
    <div style={{ overflowY: "auto", padding: "8px 10px" }}>
      <input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder="Filter files…"
        aria-label="filter repo files"
        style={{ width: "100%", boxSizing: "border-box", fontSize: 11, marginBottom: 8, padding: "3px 6px", background: "var(--surface2)", border: "1px solid var(--border)", borderRadius: 4, color: "var(--text)" }}
      />
      {paths.length === 0 ? (
        <div style={{ fontSize: 12, color: "var(--text-muted)" }}>No files.</div>
      ) : (
        renderNodes(tree, 0)
      )}
    </div>
  );
}
