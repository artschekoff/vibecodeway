import { describe, it, expect } from "vitest";
import { applyPlaceholders } from "./terminalPlaceholders";

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
