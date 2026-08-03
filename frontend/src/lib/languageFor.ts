// Registry-driven Monaco language detection.
//
// This module deliberately imports NO `monaco-editor`: the registry data is
// pushed in via `setLanguageRegistry()` (called once from setupMonaco with
// `monaco.languages.getLanguages()`), keeping the helper pure and cheap to
// unit-test in jsdom.

/** Minimal shape of a Monaco language registry entry (structurally matches
 * monaco's `ILanguageExtensionPoint`). */
export type LanguageRegistryEntry = {
  id: string;
  extensions?: string[];
  filenames?: string[];
};

// Fallback map used before setLanguageRegistry runs (and in fallback tests):
// the original 14 entries plus mjs/cjs → javascript and mts/cts → typescript,
// so the common languages highlight even without the live registry.
const FALLBACK: Record<string, string> = {
  ts: "typescript", tsx: "typescript", mts: "typescript", cts: "typescript",
  js: "javascript", jsx: "javascript", mjs: "javascript", cjs: "javascript",
  go: "go", sql: "sql", json: "json", css: "css", html: "html", md: "markdown",
  py: "python", sh: "shell", yml: "yaml", yaml: "yaml",
};

// Module-level lookups built from the injected registry. Keys are lowercased.
// extToLang keys include the leading dot (e.g. ".mjs"); filenameToLang keys are
// bare filenames (e.g. "dockerfile").
let extToLang: Record<string, string> | null = null;
let filenameToLang: Record<string, string> | null = null;

/** Build the extension/filename lookups from a Monaco language registry.
 * When several languages claim the same extension/filename, the first one wins
 * (matching Monaco's own registration precedence). */
export function setLanguageRegistry(langs: LanguageRegistryEntry[]): void {
  const exts: Record<string, string> = {};
  const names: Record<string, string> = {};
  for (const lang of langs) {
    for (const ext of lang.extensions ?? []) {
      const key = ext.toLowerCase();
      if (!(key in exts)) exts[key] = lang.id;
    }
    for (const name of lang.filenames ?? []) {
      const key = name.toLowerCase();
      if (!(key in names)) names[key] = lang.id;
    }
  }
  extToLang = exts;
  filenameToLang = names;
}

/** Reset the injected registry so the built-in fallback map is used again.
 * Exported for tests: module-level state persists between tests in one file,
 * so the fallback cases call this to clear any registry a prior test injected. */
export function resetLanguageRegistry(): void {
  extToLang = null;
  filenameToLang = null;
}

/** Resolve a Monaco language id for a file path.
 * Order: (1) exact filename match, (2) extension match, (3) "plaintext". */
export function languageFor(path: string): string {
  const basename = path.split("/").pop() ?? "";
  const lowerName = basename.toLowerCase();

  if (extToLang && filenameToLang) {
    const byName = filenameToLang[lowerName];
    if (byName) return byName;
    const ext = lowerName.split(".").pop() ?? "";
    const byExt = extToLang["." + ext];
    if (byExt) return byExt;
    return "plaintext";
  }

  // Fallback: extension-only lookup against the built-in map.
  const ext = lowerName.split(".").pop() ?? "";
  return FALLBACK[ext] ?? "plaintext";
}
