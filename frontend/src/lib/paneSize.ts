// Pane-size math for the resizable editor panes. Pure + framework-free so it
// can be unit-tested without React or a DOM.

export function clampPaneSize(size: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, size));
}

// nextPaneSize applies a drag delta (px) to the current size and clamps it.
export function nextPaneSize(current: number, delta: number, min: number, max: number): number {
  return clampPaneSize(current + delta, min, max);
}

// readStoredPaneSize parses a persisted string, falling back to `initial` when
// it is absent or not a finite integer, and clamps the result to [min, max].
export function readStoredPaneSize(raw: string | null, initial: number, min: number, max: number): number {
  const n = raw == null ? NaN : Number.parseInt(raw, 10);
  return Number.isFinite(n) ? clampPaneSize(n, min, max) : clampPaneSize(initial, min, max);
}

// fitPaneSize caps a stored pane size to what its container can actually give,
// leaving `minOther` px for the pane it shares the axis with.
//
// This is the guard that keeps a resize handle reachable. Without it a stored
// size larger than the window collapses the neighbouring pane to zero and
// pushes the handle off the top of the layout, so the user can enlarge a pane
// but never shrink it again — recoverable only by clearing localStorage.
//
// The stored preference is deliberately NOT rewritten: callers persist what the
// user chose and render what fits, so the original size comes back when the
// window grows. containerSize <= 0 means "not measured yet" — return the stored
// size rather than flashing the pane to its minimum on first paint.
export function fitPaneSize(stored: number, containerSize: number, minOther: number, min: number): number {
  if (containerSize <= 0) return stored;
  return clampPaneSize(stored, min, Math.max(min, containerSize - minOther));
}
