/// <reference types="vite/client" />

// monaco-editor@0.56.0's package.json `typings` field points to a
// nonexistent `esm/vs/editor/editor.main.d.ts` (upstream packaging mismatch;
// the real declarations for the ESM entry live at `esm/vs/index.d.ts`).
// Redirect `import ... from "monaco-editor"` to the real type declarations.
declare module "monaco-editor" {
  export * from "monaco-editor/esm/vs/index";
}
