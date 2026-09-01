import '@testing-library/jest-dom/vitest';

// Node >= 22 exposes a global `localStorage` (experimental Web Storage API).
// Without `--localstorage-file` its methods throw, and it shadows jsdom's
// window.localStorage, so any component reading localStorage fails under
// vitest. Install a simple in-memory implementation.
if (typeof localStorage === 'undefined' || typeof localStorage.getItem !== 'function') {
  let store: Record<string, string> = {};
  const mock: Storage = {
    getItem: (k) => (k in store ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v); },
    removeItem: (k) => { delete store[k]; },
    clear: () => { store = {}; },
    key: (i) => Object.keys(store)[i] ?? null,
    get length() { return Object.keys(store).length; },
  };
  Object.defineProperty(globalThis, 'localStorage', { value: mock, configurable: true });
}

// jsdom does not implement document.queryCommandSupported, which
// monaco-editor's clipboard module calls at module-load time.
if (typeof document.queryCommandSupported !== 'function') {
  document.queryCommandSupported = () => false;
}

// jsdom does not implement ResizeObserver, which the editor uses to keep panes
// inside their container. jsdom also does no layout, so a real implementation
// would have nothing to report — a stub that never fires is the honest double:
// components fall back to their unmeasured behaviour, which is what these tests
// assert against.
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}
