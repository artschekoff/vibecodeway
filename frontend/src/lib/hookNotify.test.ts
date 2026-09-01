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

describe("hookNotificationOptions — notification rules", () => {
  it("leads with the condition and shows the user's note verbatim", () => {
    const opt = hookNotificationOptions({
      contextId: "c1",
      contextKey: "PROJ-1",
      eventCategory: "jira_update",
      conditionText: 'Condition met: Jira status "In Review" → "Done"',
      note: "tell QA we are ready",
    });

    expect(opt.title).toContain("PROJ-1");
    expect(opt.body).toContain('Condition met: Jira status "In Review" → "Done"');
    expect(opt.body).toContain("tell QA we are ready");
  });

  it("falls back to the generic event text when no rule matched", () => {
    const opt = hookNotificationOptions({
      contextId: "c1",
      contextKey: "PROJ-1",
      eventCategory: "jira_update",
      conditionText: "",
      note: "",
    });

    expect(opt.body).toContain("Jira update");
    expect(opt.body).not.toContain("Condition met");
  });

  it("still shows the note when the condition line is missing", () => {
    const opt = hookNotificationOptions({
      contextId: "c1",
      contextKey: "PROJ-1",
      eventCategory: "jira_update",
      note: "tell QA we are ready",
    });

    expect(opt.body).toContain("tell QA we are ready");
  });
});
