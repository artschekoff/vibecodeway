import { describe, it, expect } from "vitest";
import { clampPaneSize, nextPaneSize, readStoredPaneSize, fitPaneSize } from "./paneSize";

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

describe("fitPaneSize", () => {
  it("returns the stored size when it fits", () => {
    expect(fitPaneSize(260, 800, 80, 90)).toBe(260);
  });

  it("caps the size to what the container can actually give", () => {
    // The whole reason this exists: a pane taller than its container pushes the
    // resize handle off-screen, and the user can never drag it back.
    expect(fitPaneSize(900, 500, 80, 90)).toBe(420);
  });

  it("never returns less than the pane minimum, even in a tiny container", () => {
    expect(fitPaneSize(900, 100, 80, 90)).toBe(90);
    expect(fitPaneSize(900, 0.0001, 80, 90)).toBe(90);
  });

  it("returns the stored size untouched before the container has been measured", () => {
    // 0 means "not measured yet" — clamping then would flash the pane to its
    // minimum on first paint.
    expect(fitPaneSize(260, 0, 80, 90)).toBe(260);
  });

  it("does not mutate the stored preference, so it returns when the window grows", () => {
    const stored = 900;
    expect(fitPaneSize(stored, 500, 80, 90)).toBe(420);
    expect(fitPaneSize(stored, 1200, 80, 90)).toBe(900);
  });
});
