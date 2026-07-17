import { describe, expect, it } from "vitest";
import { workspaceMatchesQuery, branchMatchesQuery } from "./repoFilter";

describe("workspaceMatchesQuery", () => {
  const w = { branch: "feat_JH-4264_api-update-topline-card-labels", worktreePath: "/repo/.git/worktrees/feat_JH-4264" };
  it("matches everything when query is empty", () => {
    expect(workspaceMatchesQuery(w, "")).toBe(true);
  });
  it("matches the branch name substring", () => {
    expect(workspaceMatchesQuery(w, "4264")).toBe(true);
    expect(workspaceMatchesQuery(w, "topline")).toBe(true);
  });
  it("matches the worktree folder basename", () => {
    expect(workspaceMatchesQuery(w, "worktrees")).toBe(false); // full path is NOT matched
    expect(workspaceMatchesQuery({ branch: "x", worktreePath: "/a/b/my-folder" }, "my-folder")).toBe(true);
  });
  it("is case-insensitive on the row text (query already lowercased)", () => {
    expect(workspaceMatchesQuery({ branch: "Feat-ABC", worktreePath: "/a/B" }, "abc")).toBe(true);
  });
  it("returns false when nothing matches", () => {
    expect(workspaceMatchesQuery(w, "nonexistent")).toBe(false);
  });
});

describe("branchMatchesQuery", () => {
  it("matches everything when query is empty", () => {
    expect(branchMatchesQuery({ name: "main" }, "")).toBe(true);
  });
  it("matches the branch name substring, case-insensitive", () => {
    expect(branchMatchesQuery({ name: "feat_JH-4264" }, "4264")).toBe(true);
    expect(branchMatchesQuery({ name: "Feat-ABC" }, "abc")).toBe(true);
  });
  it("returns false when nothing matches", () => {
    expect(branchMatchesQuery({ name: "main" }, "4264")).toBe(false);
  });
});
