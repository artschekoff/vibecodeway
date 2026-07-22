import { describe, it, expect } from "vitest";
import { splitInlineCode } from "./inlineRefs";

describe("splitInlineCode", () => {
  it("splits inline code spans from plain text", () => {
    expect(splitInlineCode("fix `auth.go:42` now")).toEqual([
      { code: false, value: "fix " },
      { code: true, value: "auth.go:42" },
      { code: false, value: " now" },
    ]);
  });
  it("returns a single text segment when no code spans", () => {
    expect(splitInlineCode("plain")).toEqual([{ code: false, value: "plain" }]);
  });
});
