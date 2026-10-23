/// <reference types="vite/client" />

// monaco-editor@0.56.0's package.json `typings` field points to a
// nonexistent `esm/vs/editor/editor.main.d.ts` (upstream packaging mismatch;
// the real declarations for the ESM entry live at `esm/vs/index.d.ts`).
// Redirect `import ... from "monaco-editor"` to the real type declarations.
declare module "monaco-editor" {
  export * from "monaco-editor/esm/vs/index";
}

// monaco-vim ships no type declarations. initVimMode binds vim keymaps to a
// Monaco editor and renders the ex/status line into the given node; VimMode.Vim
// is the CodeMirror vim API (handleKey feeds it a key programmatically).
declare module "monaco-vim" {
  export function initVimMode(editor: unknown, statusNode: HTMLElement): { dispose: () => void };
  export const VimMode: { Vim?: { handleKey?: (cm: unknown, key: string) => void } };
}
