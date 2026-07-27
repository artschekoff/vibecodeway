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
