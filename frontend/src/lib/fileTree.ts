// Build a VSCode-style file tree from a flat list of "/"-separated paths.
// Directories sort before files, both alphabetically (case-insensitive).
// With compact=true, single-child folder chains collapse into one node
// ("src/functions/cf" instead of src › functions › cf) like VSCode's explorer.

export type FileTreeNode =
  | { type: "file"; name: string; path: string }
  | { type: "dir"; name: string; path: string; children: FileTreeNode[] };

type DirBuild = { dirs: Map<string, DirBuild>; files: Map<string, string> };

const byName = (a: [string, unknown], b: [string, unknown]) =>
  a[0].localeCompare(b[0], undefined, { sensitivity: "base" });

function toNodes(build: DirBuild, prefix: string): FileTreeNode[] {
  const dirs: FileTreeNode[] = [...build.dirs.entries()].sort(byName).map(([name, sub]) => {
    const path = prefix ? `${prefix}/${name}` : name;
    return { type: "dir", name, path, children: toNodes(sub, path) };
  });
  const files: FileTreeNode[] = [...build.files.entries()].sort(byName).map(([name, full]) => ({
    type: "file", name, path: full,
  }));
  return [...dirs, ...files];
}

// Merge a directory with its single child directory (no sibling files), repeatedly.
function compact(node: FileTreeNode): FileTreeNode {
  if (node.type === "file") return node;
  let cur = node;
  while (cur.children.length === 1 && cur.children[0].type === "dir") {
    const child = cur.children[0];
    cur = { type: "dir", name: `${cur.name}/${child.name}`, path: child.path, children: child.children };
  }
  return { ...cur, children: cur.children.map(compact) };
}

export function buildFileTree(paths: string[], compactFolders = true): FileTreeNode[] {
  const root: DirBuild = { dirs: new Map(), files: new Map() };
  for (const full of paths) {
    const parts = full.split("/").filter(Boolean);
    if (parts.length === 0) continue;
    let node = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const d = parts[i];
      let sub = node.dirs.get(d);
      if (!sub) { sub = { dirs: new Map(), files: new Map() }; node.dirs.set(d, sub); }
      node = sub;
    }
    node.files.set(parts[parts.length - 1], full);
  }
  const nodes = toNodes(root, "");
  return compactFolders ? nodes.map(compact) : nodes;
}
