import { describe, it, expect } from "vitest";
import {
  openFileReducer, pinFileReducer, closeOthersFilesReducer, tabKey, type OpenTab, type EditorFilesState,
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

  it("closeOthersFilesReducer keeps only the kept tab and makes it active", () => {
    const s = closeOthersFilesReducer(seed(), tabKey(mk("b.ts")));
    expect(s.tabs.map((t) => t.path)).toEqual(["b.ts"]);
    expect(s.activeKey).toBe(tabKey(mk("b.ts")));
  });

  it("closeOthersFilesReducer drops pinned tabs too — only the kept one remains", () => {
    let s = pinFileReducer(seed(), tabKey(mk("a.ts")), true); // pin a
    s = closeOthersFilesReducer(s, tabKey(mk("c.ts"))); // keep c
    expect(s.tabs.map((t) => t.path)).toEqual(["c.ts"]);
  });

  it("closeOthersFilesReducer empties everything when the kept key is absent", () => {
    const s = closeOthersFilesReducer(seed(), "nope::nope");
    expect(s.tabs).toEqual([]);
    expect(s.activeKey).toBe("");
  });
});
