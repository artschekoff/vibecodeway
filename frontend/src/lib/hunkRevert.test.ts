import { describe, it, expect } from "vitest";
import { computeHunkRevert } from "./hunkRevert";

const ORIG = ["a", "b", "c", "d"]; // base file lines 1..4

describe("computeHunkRevert", () => {
  it("modify: replaces modified lines with the original lines", () => {
    // original lines 2..3 ("b","c") changed to modified lines 2..2 ("B")
    const r = computeHunkRevert(ORIG, {
      originalStartLineNumber: 2, originalEndLineNumber: 3,
      modifiedStartLineNumber: 2, modifiedEndLineNumber: 2,
    });
    expect(r).toEqual({ removeFromLine: 2, removeToLine: 2, insertLines: ["b", "c"] });
  });

  it("pure addition: deletes the added modified lines, inserts nothing", () => {
    // nothing in original (endLine 0), modified lines 3..4 were added
    const r = computeHunkRevert(ORIG, {
      originalStartLineNumber: 2, originalEndLineNumber: 0,
      modifiedStartLineNumber: 3, modifiedEndLineNumber: 4,
    });
    expect(r).toEqual({ removeFromLine: 3, removeToLine: 4, insertLines: [] });
  });

  it("pure deletion: re-inserts the original lines after the modified anchor", () => {
    // original lines 2..3 ("b","c") were deleted; modified anchor line 1
    const r = computeHunkRevert(ORIG, {
      originalStartLineNumber: 2, originalEndLineNumber: 3,
      modifiedStartLineNumber: 1, modifiedEndLineNumber: 0,
    });
    // insert after modified line 1 => removeFromLine 2, removeToLine 1 (delete nothing)
    expect(r).toEqual({ removeFromLine: 2, removeToLine: 1, insertLines: ["b", "c"] });
  });

  it("deletion at the very top (anchor 0): inserts before modified line 1", () => {
    const r = computeHunkRevert(ORIG, {
      originalStartLineNumber: 1, originalEndLineNumber: 1,
      modifiedStartLineNumber: 0, modifiedEndLineNumber: 0,
    });
    expect(r).toEqual({ removeFromLine: 1, removeToLine: 0, insertLines: ["a"] });
  });
});
