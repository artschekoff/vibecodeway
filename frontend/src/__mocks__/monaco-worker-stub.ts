// Stub for monaco-editor's `?worker` Vite asset imports under vitest.
// Vitest transforms test modules via Vite's SSR pipeline, which does not run
// the client-only `?worker` plugin that Vite's real dev/build pipeline uses
// to turn these specifiers into Worker constructors. This stub stands in for
// that default export so `monacoSetup.ts` can be imported under test; the
// actual `?worker` imports are still used by the real Vite dev/build (Wails
// renderer) pipeline — see vitest.config.ts alias entries.
export default class MonacoWorkerStub {}
