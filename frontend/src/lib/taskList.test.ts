import { describe, it, expect } from "vitest";
import { parseBody, serialize, toggleAt, addStep } from "./taskList";

describe("taskList", () => {
  it("parses task and text lines", () => {
    const body = "Intro line\n- [ ] do A\n- [x] do B `f.go:1`";
    const lines = parseBody(body);
    expect(lines[0]).toEqual({ kind: "text", raw: "Intro line" });
    expect(lines[1]).toEqual({ kind: "task", checked: false, text: "do A" });
    expect(lines[2]).toEqual({ kind: "task", checked: true, text: "do B `f.go:1`" });
  });

  it("round-trips via serialize", () => {
    const body = "x\n- [ ] a\n- [x] b";
    expect(serialize(parseBody(body))).toBe(body);
  });

  it("toggles the Nth task only (ignores text lines)", () => {
    const body = "note\n- [ ] a\n- [ ] b";
    expect(toggleAt(body, 1)).toBe("note\n- [ ] a\n- [x] b");
  });

  it("appends a new empty step", () => {
    expect(addStep("- [x] a")).toBe("- [x] a\n- [ ] ");
  });

  it("treats a body with no task lines as all text", () => {
    const body = "just prose\nsecond line";
    expect(parseBody(body).every((l) => l.kind === "text")).toBe(true);
  });
});
