import { describe, it, expect } from "vitest";
import {
  openFileReducer, pinFileReducer, closeAllFilesReducer, tabKey, type OpenTab, type EditorFilesState,
} from "./EditorFilesContext";

const mk = (path: string): OpenTab => ({ repoPath: "/r", repoName: "acme/r", path, status: "modified" });

function seed(): EditorFilesState {
  let s: EditorFilesState = { tabs: [], activeKey: "" };
  s = openFileReducer(s, mk("a.ts"));
  s = openFileReducer(s, mk("b.ts"));
  s = openFileReducer(s, mk("c.ts"));
  return s; // active = c.ts
}

describe("editor file-tab reducers", () => {
  it("pinFileReducer sets pinned on the matching tab only", () => {
    const s = pinFileReducer(seed(), tabKey(mk("b.ts")), true);
    expect(s.tabs.find((t) => t.path === "b.ts")?.pinned).toBe(true);
    expect(s.tabs.find((t) => t.path === "a.ts")?.pinned).toBeFalsy();
  });

  it("closeAllFilesReducer keeps only pinned tabs", () => {
    let s = pinFileReducer(seed(), tabKey(mk("a.ts")), true);
    s = closeAllFilesReducer(s);
    expect(s.tabs.map((t) => t.path)).toEqual(["a.ts"]);
    expect(s.activeKey).toBe(tabKey(mk("a.ts"))); // active re-targeted to the survivor
  });

  it("closeAllFilesReducer empties everything when nothing is pinned", () => {
    const s = closeAllFilesReducer(seed());
    expect(s.tabs).toEqual([]);
    expect(s.activeKey).toBe("");
  });

  it("closeAllFilesReducer keeps the active key if the active tab is pinned", () => {
    let s = pinFileReducer(seed(), tabKey(mk("c.ts")), true); // c is active
    s = closeAllFilesReducer(s);
    expect(s.activeKey).toBe(tabKey(mk("c.ts")));
  });
});
