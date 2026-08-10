import { describe, it, expect } from "vitest";
import { eventToToken, parseChord, serializeChord, formatChord } from "./chord";

const ev = (o: Partial<KeyboardEvent>) => o as KeyboardEvent;

describe("chord", () => {
  it("maps metaKey to mod on darwin and ctrlKey to mod on other", () => {
    expect(eventToToken(ev({ metaKey: true, key: "t" }), "darwin")).toEqual(
      { mod: true, ctrl: false, shift: false, alt: false, key: "t" });
    expect(eventToToken(ev({ ctrlKey: true, key: "t" }), "other")).toEqual(
      { mod: true, ctrl: false, shift: false, alt: false, key: "t" });
  });

  it("keeps raw ctrl separate from mod on darwin (for zone focus)", () => {
    expect(eventToToken(ev({ ctrlKey: true, key: "ArrowRight" }), "darwin")).toEqual(
      { mod: false, ctrl: true, shift: false, alt: false, key: "arrowright" });
  });

  it("returns null when only a modifier is pressed", () => {
    expect(eventToToken(ev({ metaKey: true, key: "Meta" }), "darwin")).toBeNull();
  });

  it("round-trips through serialize/parse", () => {
    const c = parseChord("mod+shift+[");
    expect(serializeChord(c)).toBe("mod+shift+[");
    expect(c.key).toBe("[");
  });

  it("formats for display per platform", () => {
    expect(formatChord(parseChord("mod+shift+["), "darwin")).toBe("⌘⇧[");
    expect(formatChord(parseChord("ctrl+arrowup"), "other")).toBe("Ctrl+↑");
  });
});
