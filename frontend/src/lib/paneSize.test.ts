import { describe, it, expect } from "vitest";
import { clampPaneSize, nextPaneSize, readStoredPaneSize } from "./paneSize";

describe("clampPaneSize", () => {
  it("clamps below min, above max, and passes through in-range", () => {
    expect(clampPaneSize(10, 100, 400)).toBe(100);
    expect(clampPaneSize(999, 100, 400)).toBe(400);
    expect(clampPaneSize(250, 100, 400)).toBe(250);
  });
});

describe("nextPaneSize", () => {
  it("adds the delta then clamps to range", () => {
    expect(nextPaneSize(300, -50, 180, 640)).toBe(250);
    expect(nextPaneSize(300, 1000, 180, 640)).toBe(640);
    expect(nextPaneSize(200, -1000, 180, 640)).toBe(180);
  });
});

describe("readStoredPaneSize", () => {
  it("falls back to initial when the stored value is missing or unparseable", () => {
    expect(readStoredPaneSize(null, 300, 180, 640)).toBe(300);
    expect(readStoredPaneSize("abc", 300, 180, 640)).toBe(300);
  });
  it("parses a stored value and clamps it", () => {
    expect(readStoredPaneSize("250", 300, 180, 640)).toBe(250);
    expect(readStoredPaneSize("9999", 300, 180, 640)).toBe(640);
  });
});
