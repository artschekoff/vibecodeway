import { describe, it, expect } from "vitest";
import { applyPlaceholders, withAgentHydration } from "./terminalPlaceholders";

describe("applyPlaceholders", () => {
  it("replaces a known token", () => {
    expect(applyPlaceholders("/load-context <context-name>", { "context-name": "tell-us-more-fin" }))
      .toBe("/load-context tell-us-more-fin");
  });
  it("replaces every occurrence", () => {
    expect(applyPlaceholders("<context-name> <context-name>", { "context-name": "x" })).toBe("x x");
  });
  it("leaves unknown tokens untouched", () => {
    expect(applyPlaceholders("a <unknown> b", { "context-name": "x" })).toBe("a <unknown> b");
  });
  it("substitutes an empty string for a missing value", () => {
    expect(applyPlaceholders("<context-name>!", { "context-name": "" })).toBe("!");
  });
});

describe("withAgentHydration", () => {
  const h = 'You work in context "hco". Run: vibecodeway-cli context-get --context-key hco';
  it("substitutes an explicit placeholder", () => {
    const out = withAgentHydration("cursor-agent --model x <context-hydration>", h);
    expect(out.startsWith("cursor-agent --model x \"")).toBe(true);
    expect(out).not.toContain("<context-hydration>");
  });
  it("substitutes the placeholder for any agent command (codex)", () => {
    expect(withAgentHydration("codex <context-hydration>", h)).toBe(`codex "${h.replace(/"/g, '\\"')}"`);
  });
  it("leaves commands without the placeholder untouched — no magic agent detection", () => {
    expect(withAgentHydration("git status", h)).toBe("git status");
    expect(withAgentHydration("cursor-agent", h)).toBe("cursor-agent");
    expect(withAgentHydration("codex", h)).toBe("codex");
  });
  it("drops the placeholder when hydration is empty", () => {
    expect(withAgentHydration("cursor-agent <context-hydration>", "")).toBe("cursor-agent ");
  });
});
