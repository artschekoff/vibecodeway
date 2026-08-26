import { describe, it, expect } from "vitest";
import { needsAttentionTiles } from "./needsAttentionPanel";
import type { Context } from "../types";

function make(over: Partial<Context>): Context {
  return {
    id: "c1", key: "k", status: "active", orgId: "o1", createdAt: "", updatedAt: "",
    snapshotStatus: "needs_attention", snapshotIsStale: false,
    ...over,
  };
}

describe("needsAttentionTiles", () => {
  it("keeps active contexts that still need attention", () => {
    const c = make({ id: "live" });
    expect(needsAttentionTiles([c])).toEqual([c]);
  });

  it("drops archived contexts — a closed-out context is nobody's move", () => {
    expect(needsAttentionTiles([make({ id: "old", status: "archived" })])).toEqual([]);
  });

  it("drops contexts in any other status", () => {
    expect(needsAttentionTiles([make({ snapshotStatus: "in_progress" })])).toEqual([]);
  });
});
