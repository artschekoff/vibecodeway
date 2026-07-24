import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";

let done = false;

// Workers are constructed via `new URL(..., import.meta.url)` rather than
// Vite's `?worker` import suffix: Vite 3/Rollup's worker plugin resolves the
// literal through the same node "exports" resolution Node itself uses, and
// monaco-editor@0.56.0's package.json only exposes subpaths via
// `"./*": "./esm/vs/*.js"` (no `esm/vs/` prefix and no `.js` suffix in the
// specifier itself — both are injected by the exports map). The on-disk path
// (`monaco-editor/esm/vs/....js`) and an explicit `....js` suffix both fail
// to resolve under Vite 3.2's exports handling; the extensionless form below
// is the one that matches. This still statically bundles the worker as a
// local asset — no CDN, no external host (required: the Wails webview's CSP
// blocks external hosts). The Rollup worker plugin only recognizes a string
// literal (not a template/dynamic string) as the first argument, so each
// worker needs its own literal `new URL(...)` call.
function getEditorWorker(): Worker {
  return new Worker(new URL("monaco-editor/editor/editor.worker", import.meta.url), {
    type: "module",
  });
}

function getJsonWorker(): Worker {
  return new Worker(new URL("monaco-editor/language/json/json.worker", import.meta.url), {
    type: "module",
  });
}

function getCssWorker(): Worker {
  return new Worker(new URL("monaco-editor/language/css/css.worker", import.meta.url), {
    type: "module",
  });
}

function getHtmlWorker(): Worker {
  return new Worker(new URL("monaco-editor/language/html/html.worker", import.meta.url), {
    type: "module",
  });
}

function getTsWorker(): Worker {
  return new Worker(new URL("monaco-editor/language/typescript/ts.worker", import.meta.url), {
    type: "module",
  });
}

// setupMonaco wires @monaco-editor/react to the bundled monaco (no CDN — the
// Wails webview blocks external hosts) and registers language workers.
export function setupMonaco(): void {
  if (done) return;
  done = true;

  (self as unknown as { MonacoEnvironment: monaco.Environment }).MonacoEnvironment = {
    getWorker(_moduleId: string, label: string) {
      switch (label) {
        case "json":
          return getJsonWorker();
        case "css":
        case "scss":
        case "less":
          return getCssWorker();
        case "html":
        case "handlebars":
        case "razor":
          return getHtmlWorker();
        case "typescript":
        case "javascript":
          return getTsWorker();
        default:
          return getEditorWorker();
      }
    },
  };

  loader.config({ monaco });
}
