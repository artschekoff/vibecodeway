import { describe, it, expect } from "vitest";
import { COMMANDS, commandById, commandLabel } from "./registry";

describe("registry", () => {
  it("every command id equals object.action and is unique", () => {
    const ids = new Set<string>();
    for (const c of COMMANDS) {
      expect(c.id).toBe(`${c.object}.${c.action}`);
      expect(ids.has(c.id)).toBe(false);
      ids.add(c.id);
    }
  });
  it("declares the required commands", () => {
    for (const id of ["search.global", "editor.open", "help.open", "terminal.new",
      "terminal.here", "terminal.insert", "files.reload", "files.focusSearch",
      "pr.open", "pr.reload", "context.copyName", "sidebar.next", "sidebar.prev"]) {
      expect(commandById(id), id).toBeTruthy();
    }
  });
  it("builds a leading_button · object · action label", () => {
    expect(commandLabel(commandById("terminal.new")!, "darwin")).toBe("⌘T · Terminal · New");
  });
  it("each object with a leader has exactly one defaultAction", () => {
    const byObject: Record<string, number> = {};
    for (const c of COMMANDS.filter((c) => c.defaultAction)) byObject[c.object] = (byObject[c.object] ?? 0) + 1;
    for (const n of Object.values(byObject)) expect(n).toBe(1);
  });
});
