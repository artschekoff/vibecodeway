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

// worktreeForPath picks the repoPath that is the longest-matching prefix of
// path (a model URI's .path, e.g. from file://<worktree>/<relpath>). Returns
// null when none of the known repo paths contain it — e.g. the synthetic
// ".orig" original-side model of a diff editor.
export function worktreeForPath(repoPaths: string[], path: string): string | null {
  let best: string | null = null;
  for (const repoPath of repoPaths) {
    if (repoPath && (path === repoPath || path.startsWith(`${repoPath}/`))) {
      if (best === null || repoPath.length > best.length) best = repoPath;
    }
  }
  return best;
}

export interface SymbolNavDeps {
  getRepoPath: () => string;
  getRepoPaths: () => string[];
  getContextId: () => string;
  getRepoName: () => string;
  openFile: (contextId: string, tab: { repoPath: string; repoName: string; path: string; status: string; revealLine: number }) => void;
}

let done = false;
// activeDeps always points at the currently visible EditorWorkspace's deps.
// Providers/opener are registered once (module-level guard below) but read
// this at invocation time, so with multiple mounted (hidden + visible) tabs
// navigation always targets the one the user is actually looking at.
let activeDeps: SymbolNavDeps | null = null;

// registerMonacoSymbols wires tags-based navigation into Monaco for every
// language: definition (F12 / ⌘-click / vim gd), references (⇧F12) and an
// editor opener that routes cross-file targets through the app's file tabs.
// Call on every mount/visibility change of the active workspace — it always
// updates activeDeps, but only registers monaco's providers/opener once.
// releaseMonacoSymbols drops the shared deps IF they still belong to the
// releasing workspace — called on unmount so a closed editor tab's closures
// (contextId, file opener) are not retained by the module-level slot. A newer
// workspace's deps are left untouched.
export function releaseMonacoSymbols(deps: SymbolNavDeps): void {
  if (activeDeps === deps) activeDeps = null;
}

export function registerMonacoSymbols(deps: SymbolNavDeps): SymbolNavDeps {
  activeDeps = deps;
  if (done) return deps;
  done = true;

  monaco.languages.registerDefinitionProvider("*", {
    async provideDefinition(model, position) {
      const deps = activeDeps;
      const word = model.getWordAtPosition(position);
      const repoPath = deps ? (worktreeForPath(deps.getRepoPaths(), model.uri.path) ?? deps.getRepoPath()) : "";
      if (!deps || !word || !repoPath) return null;
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
      const deps = activeDeps;
      const word = model.getWordAtPosition(position);
      const repoPath = deps ? (worktreeForPath(deps.getRepoPaths(), model.uri.path) ?? deps.getRepoPath()) : "";
      if (!deps || !word || !repoPath) return null;
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
      const deps = activeDeps;
      if (!deps) return false;
      const repoPath = worktreeForPath(deps.getRepoPaths(), resource.path) ?? deps.getRepoPath();
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
  return deps;
}
