import '@testing-library/jest-dom/vitest';

// jsdom does not implement document.queryCommandSupported, which
// monaco-editor's clipboard module calls at module-load time.
if (typeof document.queryCommandSupported !== 'function') {
  document.queryCommandSupported = () => false;
}
