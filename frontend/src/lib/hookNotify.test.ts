import { describe, it, expect } from "vitest";
import { hookNotificationOptions } from "./hookNotify";

describe("hookNotificationOptions", () => {
  it("builds a notification titled with the context key", () => {
    const opt = hookNotificationOptions({ contextId: "c1", contextKey: "PROJ-1", eventCategory: "pr_update" });
    expect(opt.title).toContain("PROJ-1");
    expect(opt.body.length).toBeGreaterThan(0);
    expect(opt.id).toContain("c1");
  });
});
