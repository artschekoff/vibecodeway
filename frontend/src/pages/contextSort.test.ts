import { describe, it, expect } from "vitest";
import { sortContexts } from "./contextSort";
import type { Context } from "../types";

const ctx = (over: Partial<Context>): Context => ({
  id: "id", key: "k", status: "active", orgId: "o", createdAt: "", updatedAt: "",
  snapshotStatus: "", snapshotIsStale: false, ...over,
});

describe("sortContexts", () => {
  const list = [
    ctx({ id: "1", key: "bravo", issueCount: 5, unviewedCount: 2, updatedAt: "2026-09-23T10:00:00Z", importance: 3.0, status: "archived" }),
    ctx({ id: "2", key: "alpha", issueCount: 1, unviewedCount: 9, updatedAt: "2026-09-21T10:00:00Z", importance: 4.0 }),
    ctx({ id: "3", key: "Charlie", issueCount: 3, updatedAt: "2026-09-22T10:00:00Z" }), // no importance/unread
  ];

  it("sorts by key case-insensitively", () => {
    expect(sortContexts(list, "key", "asc").map((c) => c.id)).toEqual(["2", "1", "3"]);
  });

  it("sorts numbers desc with missing values last", () => {
    expect(sortContexts(list, "importance", "desc").map((c) => c.id)).toEqual(["2", "1", "3"]);
    expect(sortContexts(list, "unread", "desc").map((c) => c.id)).toEqual(["2", "1", "3"]);
  });

  it("sorts by updated and by active flag", () => {
    expect(sortContexts(list, "updated", "desc").map((c) => c.id)).toEqual(["1", "3", "2"]);
    expect(sortContexts(list, "active", "asc")[0].id).toBe("1"); // archived first asc
  });

  it("does not mutate the input", () => {
    const before = list.map((c) => c.id);
    sortContexts(list, "key", "desc");
    expect(list.map((c) => c.id)).toEqual(before);
  });
});
