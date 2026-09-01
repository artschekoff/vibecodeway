import { describe, it, expect, beforeEach } from "vitest";
import { loadTerminals, saveTerminals } from "./terminalSession";

beforeEach(() => localStorage.clear());

describe("terminalSession", () => {
  it("round-trips a saved layout", () => {
    saveTerminals("ctx1", [
      { cwd: "/r/api", openedAt: 100 },
      { cwd: "/r/web", openedAt: 200, resume: "claude --resume s1" },
    ]);
    expect(loadTerminals("ctx1")).toEqual([
      { cwd: "/r/api", openedAt: 100 },
      { cwd: "/r/web", openedAt: 200, resume: "claude --resume s1" },
    ]);
  });

  it("keeps contexts apart", () => {
    saveTerminals("ctx1", [{ cwd: "/r/api", openedAt: 1 }]);
    expect(loadTerminals("ctx2")).toEqual([]);
  });

  it("returns an empty layout for missing, corrupt or non-array storage", () => {
    expect(loadTerminals("never-saved")).toEqual([]);
    localStorage.setItem("vibecodeway:editor:terminals:bad", "{not json");
    expect(loadTerminals("bad")).toEqual([]);
    localStorage.setItem("vibecodeway:editor:terminals:obj", '{"cwd":"/r"}');
    expect(loadTerminals("obj")).toEqual([]);
  });

  it("drops entries without a cwd", () => {
    localStorage.setItem("vibecodeway:editor:terminals:mixed",
      '[{"cwd":"/r/api","openedAt":1},{"openedAt":2}]');
    expect(loadTerminals("mixed")).toEqual([{ cwd: "/r/api", openedAt: 1 }]);
  });
});
