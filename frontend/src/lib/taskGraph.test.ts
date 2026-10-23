import { describe, it, expect } from "vitest";
import { visibleGraph, newlyUnblocked, parentChain, type GGraph, type GNode } from "./taskGraph";

const n = (key: string, p: Partial<GNode> = {}): GNode => ({
  key, summary: key + " summary", status: "To Do", type: "Task", priority: "High", priorityRank: 4,
  assignee: "", parentKey: "", mine: false, terminal: false, epic: false, blocked: false, ...p,
});

const g: GGraph = {
  provider: "jira",
  nodes: [
    n("E-1", { epic: true, type: "Epic" }),
    n("S-1", { type: "Story" }),
    n("T-1", { mine: true, type: "Subtask", blocked: true, blockedBy: ["X-1"] }),
    n("T-2", { mine: true, terminal: true, status: "Done" }),
    n("X-1"),            // blocker, not mine
    n("R-1"),            // only related
  ],
  edges: [
    { src: "T-1", dst: "S-1", kind: "parent" },
    { src: "S-1", dst: "E-1", kind: "parent" },
    { src: "X-1", dst: "T-1", kind: "blocks" },
    { src: "T-1", dst: "R-1", kind: "relates" },
  ],
  ready: [], waiting: ["T-1"],
};
const base = { onlyMine: false, hideDone: false, showRelates: false, query: "", collapsed: new Set<string>() };

describe("visibleGraph", () => {
  it("only mine keeps the epic chain and blockers, drops unrelated nodes", () => {
    const v = visibleGraph(g, { ...base, onlyMine: true });
    expect(v.nodes.map((x) => x.key).sort()).toEqual(["E-1", "S-1", "T-1", "T-2", "X-1"]);
    expect(v.edges.some((e) => e.kind === "relates")).toBe(false);
  });
  it("hide done drops terminal tasks but never epics; collapse hides descendants", () => {
    expect(visibleGraph(g, { ...base, hideDone: true }).nodes.map((x) => x.key)).not.toContain("T-2");
    const c = visibleGraph(g, { ...base, collapsed: new Set(["E-1"]) });
    expect(c.nodes.map((x) => x.key).sort()).toEqual(["E-1", "R-1", "T-2", "X-1"]);
  });
  it("search highlights without filtering", () => {
    const v = visibleGraph(g, { ...base, query: "x-1" });
    expect(v.nodes).toHaveLength(6);
    expect([...v.matches]).toEqual(["X-1"]);
  });
});

it("parentChain climbs to the epic", () => {
  expect(parentChain("T-1", g.edges)).toEqual(["S-1", "E-1"]);
});

it("newlyUnblocked diffs waiting → ready", () => {
  expect(newlyUnblocked(g, { ...g, ready: ["T-1", "T-9"], waiting: [] })).toEqual(["T-1"]);
  expect(newlyUnblocked(null, g)).toEqual([]);
});
