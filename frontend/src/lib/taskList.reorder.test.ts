import { describe, it, expect } from "vitest";
import { moveTask } from "./taskList";

describe("moveTask", () => {
  it("moves a task from one position to another (task-index space)", () => {
    const body = "- [ ] a\n- [ ] b\n- [ ] c";
    expect(moveTask(body, 0, 2)).toBe("- [ ] b\n- [ ] c\n- [ ] a");
  });
  it("is a no-op when from === to", () => {
    const body = "- [ ] a\n- [ ] b";
    expect(moveTask(body, 1, 1)).toBe(body);
  });
});
