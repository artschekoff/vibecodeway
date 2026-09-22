import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

// baseMode picks the diff's "original" side: "branch" = vs the merge-base with
// the default branch (full branch/PR diff), "head" = vs HEAD (only uncommitted
// edits). Omitted defaults to "branch" for back-compat. See DiffEditorTab.
export type OpenTab = { repoPath: string; repoName: string; path: string; status: string; baseMode?: "branch" | "head"; pinned?: boolean; revealLine?: number; revealNonce?: number };
export const tabKey = (t: OpenTab) => `${t.repoPath}::${t.path}`;

export type EditorFilesState = { tabs: OpenTab[]; activeKey: string };
const EMPTY: EditorFilesState = { tabs: [], activeKey: "" };

// Pure transitions (exported for unit tests).
export function openFileReducer(state: EditorFilesState, tab: OpenTab): EditorFilesState {
  const key = tabKey(tab);
  const existing = state.tabs.find((t) => tabKey(t) === key);
  // revealNonce bumps every time a revealLine is supplied — even a repeat jump
  // to the same file+line — so DiffEditorTab's reveal effect (keyed on the
  // nonce, not the line number) fires again after the user scrolls away.
  const nextNonce = tab.revealLine ? (existing?.revealNonce ?? 0) + 1 : existing?.revealNonce;
  const tabs = existing
    ? state.tabs.map((t) => (tabKey(t) === key ? { ...t, revealLine: tab.revealLine, revealNonce: nextNonce } : t))
    : [...state.tabs, { ...tab, revealNonce: nextNonce }];
  return { tabs, activeKey: key };
}

export function closeFileReducer(state: EditorFilesState, key: string): EditorFilesState {
  const tabs = state.tabs.filter((t) => tabKey(t) !== key);
  const activeKey =
    state.activeKey === key ? (tabs.length ? tabKey(tabs[tabs.length - 1]) : "") : state.activeKey;
  return { tabs, activeKey };
}

export function pinFileReducer(state: EditorFilesState, key: string, pinned: boolean): EditorFilesState {
  return { ...state, tabs: state.tabs.map((t) => (tabKey(t) === key ? { ...t, pinned } : t)) };
}

// "Close all but this": keep only the tab whose key is keepKey (the right-clicked
// tab), dropping everything else — pinned included — so a single tab remains.
export function closeOthersFilesReducer(state: EditorFilesState, keepKey: string): EditorFilesState {
  const tabs = state.tabs.filter((t) => tabKey(t) === keepKey);
  return { tabs, activeKey: tabs.length ? keepKey : "" };
}

type EditorFilesContextValue = {
  getFiles: (contextId: string) => EditorFilesState;
  openEditorFile: (contextId: string, tab: OpenTab) => void;
  closeEditorFile: (contextId: string, key: string) => void;
  setActiveEditorFile: (contextId: string, key: string) => void;
  pinEditorFile: (contextId: string, key: string, pinned: boolean) => void;
  closeOtherEditorFiles: (contextId: string, keepKey: string) => void;
};

export const EditorFilesContext = createContext<EditorFilesContextValue | null>(null);

export function EditorFilesProvider({ children }: { children: ReactNode }) {
  const [byContext, setByContext] = useState<Record<string, EditorFilesState>>({});

  const getFiles = useCallback((contextId: string) => byContext[contextId] ?? EMPTY, [byContext]);

  const openEditorFile = useCallback((contextId: string, tab: OpenTab) => {
    setByContext((prev) => ({ ...prev, [contextId]: openFileReducer(prev[contextId] ?? EMPTY, tab) }));
  }, []);

  const closeEditorFile = useCallback((contextId: string, key: string) => {
    setByContext((prev) => ({ ...prev, [contextId]: closeFileReducer(prev[contextId] ?? EMPTY, key) }));
  }, []);

  const setActiveEditorFile = useCallback((contextId: string, key: string) => {
    setByContext((prev) => ({ ...prev, [contextId]: { ...(prev[contextId] ?? EMPTY), activeKey: key } }));
  }, []);

  const pinEditorFile = useCallback((contextId: string, key: string, pinned: boolean) => {
    setByContext((prev) => ({ ...prev, [contextId]: pinFileReducer(prev[contextId] ?? EMPTY, key, pinned) }));
  }, []);

  const closeOtherEditorFiles = useCallback((contextId: string, keepKey: string) => {
    setByContext((prev) => ({ ...prev, [contextId]: closeOthersFilesReducer(prev[contextId] ?? EMPTY, keepKey) }));
  }, []);

  return (
    <EditorFilesContext.Provider value={{ getFiles, openEditorFile, closeEditorFile, setActiveEditorFile, pinEditorFile, closeOtherEditorFiles }}>
      {children}
    </EditorFilesContext.Provider>
  );
}

export function useEditorFiles() {
  const ctx = useContext(EditorFilesContext);
  if (!ctx) throw new Error("useEditorFiles must be used within EditorFilesProvider");
  return ctx;
}
