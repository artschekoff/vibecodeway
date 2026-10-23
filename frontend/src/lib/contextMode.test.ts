import { describe, it, expect } from "vitest";
import { editorModeForKey, editorModeLabel } from "./contextMode";

describe("editorModeForKey", () => {
  it("detects PR-review contexts by their owner/name#number key", () => {
    expect(editorModeForKey("acme/acme-api#4521")).toBe("review");
  });

  it("treats Jira-keyed and other contexts as context editing", () => {
    expect(editorModeForKey("JH-3940")).toBe("context");
    expect(editorModeForKey("Some Context Name")).toBe("context");
    expect(editorModeForKey(undefined)).toBe("context");
    // A repo reference without a PR number is not review mode.
    expect(editorModeForKey("owner/name")).toBe("context");
  });

  it("labels each mode", () => {
    expect(editorModeLabel("review")).toBe("PR Review");
    expect(editorModeLabel("context")).toBe("Context Editing");
  });
});
