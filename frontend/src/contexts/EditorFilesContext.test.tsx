import { describe, it, expect } from "vitest";
import { openFileReducer, closeFileReducer, tabKey, type OpenTab } from "./EditorFilesContext";

const t = (path: string): OpenTab => ({ repoPath: "/r", repoName: "acme/r", path, status: "modified" });

describe("openFileReducer", () => {
  it("adds a new tab and makes it active", () => {
    const s = openFileReducer({ tabs: [], activeKey: "" }, t("a.ts"));
    expect(s.tabs.map((x) => x.path)).toEqual(["a.ts"]);
    expect(s.activeKey).toBe(tabKey(t("a.ts")));
  });
  it("does not duplicate an already-open file but re-activates it", () => {
    const first = openFileReducer({ tabs: [], activeKey: "" }, t("a.ts"));
    const withB = openFileReducer(first, t("b.ts"));
    const again = openFileReducer(withB, t("a.ts"));
    expect(again.tabs.map((x) => x.path)).toEqual(["a.ts", "b.ts"]);
    expect(again.activeKey).toBe(tabKey(t("a.ts")));
  });
});

describe("closeFileReducer", () => {
  it("removes a tab and re-activates the last remaining one when the active tab is closed", () => {
    let s = openFileReducer({ tabs: [], activeKey: "" }, t("a.ts"));
    s = openFileReducer(s, t("b.ts")); // active = b.ts
    s = closeFileReducer(s, tabKey(t("b.ts")));
    expect(s.tabs.map((x) => x.path)).toEqual(["a.ts"]);
    expect(s.activeKey).toBe(tabKey(t("a.ts")));
  });
  it("leaves activeKey untouched when a non-active tab is closed", () => {
    let s = openFileReducer({ tabs: [], activeKey: "" }, t("a.ts"));
    s = openFileReducer(s, t("b.ts")); // active = b.ts
    s = closeFileReducer(s, tabKey(t("a.ts")));
    expect(s.activeKey).toBe(tabKey(t("b.ts")));
  });
  it("empties activeKey when the last tab is closed", () => {
    let s = openFileReducer({ tabs: [], activeKey: "" }, t("a.ts"));
    s = closeFileReducer(s, tabKey(t("a.ts")));
    expect(s.tabs).toEqual([]);
    expect(s.activeKey).toBe("");
  });
});
