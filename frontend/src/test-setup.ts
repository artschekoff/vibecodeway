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
