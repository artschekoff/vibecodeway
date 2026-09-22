import { describe, it, expect } from "vitest";
import { lookupToLocations, refsToLocations, worktreeForPath } from "./monacoSymbols";

describe("monacoSymbols mapping", () => {
  it("maps symbols to file:// locations with 1-based line ranges", () => {
    const locs = lookupToLocations("/wt/api", [
      { name: "getUser", kind: "function", path: "src/user.ts", language: "TypeScript", line: 12 },
    ]);
    expect(locs).toHaveLength(1);
    expect(locs[0].uri.toString()).toBe("file:///wt/api/src/user.ts");
    expect(locs[0].range.startLineNumber).toBe(12);
  });

  it("maps references identically", () => {
    const locs = refsToLocations("/wt/api", [{ path: "a.ts", line: 3, preview: "x" }]);
    expect(locs[0].uri.path).toBe("/wt/api/a.ts");
    expect(locs[0].range.startLineNumber).toBe(3);
  });
});

describe("worktreeForPath", () => {
  it("picks the longest-matching repo path prefix", () => {
    const repoPaths = ["/wt/api", "/wt/api/nested"];
    expect(worktreeForPath(repoPaths, "/wt/api/nested/src/user.ts")).toBe("/wt/api/nested");
    expect(worktreeForPath(repoPaths, "/wt/api/src/user.ts")).toBe("/wt/api");
  });

  it("matches an exact repo path with no trailing segment", () => {
    expect(worktreeForPath(["/wt/api"], "/wt/api")).toBe("/wt/api");
  });

  it("returns null when no repo path is a prefix", () => {
    expect(worktreeForPath(["/wt/api"], "/wt/other/src/user.ts")).toBeNull();
  });

  it("ignores empty repo paths", () => {
    expect(worktreeForPath(["", "/wt/api"], "/wt/api/src/user.ts")).toBe("/wt/api");
  });
});
