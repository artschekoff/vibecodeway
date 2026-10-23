// Pure helpers for the board's task graph (provider-neutral; mirrors
// internal/taskgraph on the Go side).

export type GNode = {
  key: string;
  summary: string;
  status: string;
  type: string;
  priority: string;
  priorityRank: number;
  assignee: string;
  parentKey: string;
  mine: boolean;
  terminal: boolean;
  epic: boolean;
  blocked: boolean;
  blockedBy?: string[];
  contextId?: string;
  contextKey?: string;
  url?: string;
};
export type GEdge = { src: string; dst: string; kind: string };
export type GGraph = { provider: string; nodes: GNode[]; edges: GEdge[]; ready: string[]; waiting: string[] };

// 5 highest … 1 lowest, 0 unknown.
const PRIORITY_COLORS: Record<number, string> = {
  5: "#e05252", 4: "#f0a500", 3: "#ffcd11", 2: "#6a9a3f", 1: "#8a8f98", 0: "#5c5c5c",
};
export function priorityColor(rank: number): string {
  return PRIORITY_COLORS[rank] ?? PRIORITY_COLORS[0];
}

export function isSubtaskType(t: string): boolean {
  return t.toLowerCase().replace(/[-\s]/g, "") === "subtask";
}

export function nodeRadius(n: GNode): number {
  if (n.epic) return 22;
  return isSubtaskType(n.type) ? 9 : 13;
}

export type ViewOpts = {
  onlyMine: boolean;
  hideDone: boolean;
  showRelates: boolean;
  query: string;
  collapsed: Set<string>;
};

// parentChain returns [parent, grandparent, …] following parent edges.
export function parentChain(key: string, edges: GEdge[]): string[] {
  const parentOf = new Map<string, string>();
  for (const e of edges) if (e.kind === "parent") parentOf.set(e.src, e.dst);
  const out: string[] = [];
  let cur = parentOf.get(key);
  while (cur && out.length < 10 && !out.includes(cur)) {
    out.push(cur);
    cur = parentOf.get(cur);
  }
  return out;
}

// visibleGraph applies the toolbar filters. "Only mine" keeps my issues plus
// the chain up to their epic and their blockers, so the picture stays
// explainable. Collapsing an epic hides everything under it. The search never
// removes nodes — it returns the matching keys for highlighting.
export function visibleGraph(g: GGraph, o: ViewOpts): { nodes: GNode[]; edges: GEdge[]; matches: Set<string> } {
  let keep = new Set(g.nodes.map((n) => n.key));
  if (o.onlyMine) {
    keep = new Set();
    for (const n of g.nodes) {
      if (!n.mine) continue;
      keep.add(n.key);
      for (const a of parentChain(n.key, g.edges)) keep.add(a);
      for (const b of n.blockedBy ?? []) keep.add(b);
    }
  }
  if (o.hideDone) for (const n of g.nodes) if (n.terminal && !n.epic) keep.delete(n.key);
  if (o.collapsed.size) {
    for (const n of g.nodes) if (parentChain(n.key, g.edges).some((a) => o.collapsed.has(a))) keep.delete(n.key);
  }
  const nodes = g.nodes.filter((n) => keep.has(n.key));
  const edges = g.edges.filter((e) => keep.has(e.src) && keep.has(e.dst) && (o.showRelates || e.kind !== "relates"));
  const q = o.query.trim().toLowerCase();
  const matches = new Set<string>(
    q ? nodes.filter((n) => n.key.toLowerCase().includes(q) || n.summary.toLowerCase().includes(q)).map((n) => n.key) : [],
  );
  return { nodes, edges, matches };
}

// Node positions survive refreshes (Obsidian-style stable layout).
export const POS_KEY = "vibecodeway:board:graph:pos";
export type Positions = Record<string, { x: number; y: number }>;
export function loadPositions(): Positions {
  try { return JSON.parse(localStorage.getItem(POS_KEY) ?? "{}") as Positions; } catch { return {}; }
}
export function savePositions(p: Positions): void {
  try { localStorage.setItem(POS_KEY, JSON.stringify(p)); } catch { /* storage unavailable */ }
}

// newlyUnblocked: keys that were waiting before and are ready now.
export function newlyUnblocked(prev: GGraph | null, next: GGraph): string[] {
  if (!prev) return [];
  const w = new Set(prev.waiting);
  return next.ready.filter((k) => w.has(k));
}
