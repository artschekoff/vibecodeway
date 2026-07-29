export type LineChange = {
  originalStartLineNumber: number;
  originalEndLineNumber: number;
  modifiedStartLineNumber: number;
  modifiedEndLineNumber: number;
};

// Describes how to revert one hunk on the MODIFIED side: delete whole lines
// removeFromLine..removeToLine (inclusive) and insert insertLines at that point.
// When removeToLine < removeFromLine nothing is deleted — it is a pure insertion
// at line removeFromLine.
export type HunkRevert = { removeFromLine: number; removeToLine: number; insertLines: string[] };

export function computeHunkRevert(originalLines: string[], change: LineChange): HunkRevert {
  const origHasLines = change.originalEndLineNumber >= change.originalStartLineNumber;
  const modHasLines = change.modifiedEndLineNumber > 0;

  // The base lines this hunk should restore (empty for a pure addition).
  const insertLines = origHasLines
    ? originalLines.slice(change.originalStartLineNumber - 1, change.originalEndLineNumber)
    : [];

  if (modHasLines) {
    // Modify or pure addition: replace/delete the modified lines in place.
    return {
      removeFromLine: change.modifiedStartLineNumber,
      removeToLine: change.modifiedEndLineNumber,
      insertLines,
    };
  }
  // Pure deletion: insert the base lines just AFTER the modified anchor line.
  const at = change.modifiedStartLineNumber + 1; // anchor 0 => insert before line 1
  return { removeFromLine: at, removeToLine: at - 1, insertLines };
}
