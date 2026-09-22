import * as monaco from "monaco-editor";
import { SymbolLookup, SymbolReferences } from "../../wailsjs/go/main/App";

type Sym = { name: string; kind: string; path: string; language: string; line: number };
type Ref = { path: string; line: number; preview: string };
type Loc = { uri: monaco.Uri; range: monaco.IRange };

const lineRange = (line: number): monaco.IRange =>
  ({ startLineNumber: line, startColumn: 1, endLineNumber: line, endColumn: 1 });

export function lookupToLocations(repoPath: string, syms: Sym[]): Loc[] {
  return syms.map((s) => ({ uri: monaco.Uri.file(`${repoPath}/${s.path}`), range: lineRange(s.line) }));
}

export function refsToLocations(repoPath: string, refs: Ref[]): Loc[] {
  return refs.map((r) => ({ uri: monaco.Uri.file(`${repoPath}/${r.path}`), range: lineRange(r.line) }));
}

export interface SymbolNavDeps {
  getRepoPath: () => string;
  getContextId: () => string;
  getRepoName: () => string;
  openFile: (contextId: string, tab: { repoPath: string; repoName: string; path: string; status: string; revealLine: number }) => void;
}

let done = false;

// registerMonacoSymbols wires tags-based navigation into Monaco for every
// language: definition (F12 / ⌘-click / vim gd), references (⇧F12) and an
// editor opener that routes cross-file targets through the app's file tabs.
// Registered once; the deps getters always reflect the active editor tab.
export function registerMonacoSymbols(deps: SymbolNavDeps): void {
  if (done) return;
  done = true;

  monaco.languages.registerDefinitionProvider("*", {
    async provideDefinition(model, position) {
      const word = model.getWordAtPosition(position);
      const repoPath = deps.getRepoPath();
      if (!word || !repoPath) return null;
      try {
        const syms = (await SymbolLookup(repoPath, word.word)) as unknown as Sym[];
        return lookupToLocations(repoPath, syms ?? []);
      } catch {
        return null; // ctags-not-found or index failure: stay silent here
      }
    },
  });

  monaco.languages.registerReferenceProvider("*", {
    async provideReferences(model, position) {
      const word = model.getWordAtPosition(position);
      const repoPath = deps.getRepoPath();
      if (!word || !repoPath) return null;
      try {
        const refs = (await SymbolReferences(repoPath, word.word, 200)) as unknown as Ref[];
        return refsToLocations(repoPath, refs ?? []);
      } catch {
        return null;
      }
    },
  });

  // Cross-file jump: resolve the target through the app's own tab system.
  monaco.editor.registerEditorOpener({
    openCodeEditor(_source, resource, selectionOrPosition) {
      const repoPath = deps.getRepoPath();
      if (!repoPath || !resource.path.startsWith(`${repoPath}/`)) return false;
      const rel = resource.path.slice(repoPath.length + 1);
      const line =
        selectionOrPosition && "startLineNumber" in selectionOrPosition
          ? selectionOrPosition.startLineNumber
          : (selectionOrPosition?.lineNumber ?? 1);
      deps.openFile(deps.getContextId(), {
        repoPath, repoName: deps.getRepoName(), path: rel, status: "", revealLine: line,
      });
      return true;
    },
  });
}
