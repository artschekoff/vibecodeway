import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

export type OpenTab = { repoPath: string; repoName: string; path: string; status: string; pinned?: boolean };
export const tabKey = (t: OpenTab) => `${t.repoPath}::${t.path}`;

export type EditorFilesState = { tabs: OpenTab[]; activeKey: string };
const EMPTY: EditorFilesState = { tabs: [], activeKey: "" };

// Pure transitions (exported for unit tests).
export function openFileReducer(state: EditorFilesState, tab: OpenTab): EditorFilesState {
  const key = tabKey(tab);
  const exists = state.tabs.some((t) => tabKey(t) === key);
  return { tabs: exists ? state.tabs : [...state.tabs, tab], activeKey: key };
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

export function closeAllFilesReducer(state: EditorFilesState): EditorFilesState {
  const tabs = state.tabs.filter((t) => t.pinned);
  const activeKept = tabs.some((t) => tabKey(t) === state.activeKey);
  const activeKey = activeKept ? state.activeKey : tabs.length ? tabKey(tabs[tabs.length - 1]) : "";
  return { tabs, activeKey };
}

type EditorFilesContextValue = {
  getFiles: (contextId: string) => EditorFilesState;
  openEditorFile: (contextId: string, tab: OpenTab) => void;
  closeEditorFile: (contextId: string, key: string) => void;
  setActiveEditorFile: (contextId: string, key: string) => void;
  pinEditorFile: (contextId: string, key: string, pinned: boolean) => void;
  closeAllEditorFiles: (contextId: string) => void;
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

  const closeAllEditorFiles = useCallback((contextId: string) => {
    setByContext((prev) => ({ ...prev, [contextId]: closeAllFilesReducer(prev[contextId] ?? EMPTY) }));
  }, []);

  return (
    <EditorFilesContext.Provider value={{ getFiles, openEditorFile, closeEditorFile, setActiveEditorFile, pinEditorFile, closeAllEditorFiles }}>
      {children}
    </EditorFilesContext.Provider>
  );
}

export function useEditorFiles() {
  const ctx = useContext(EditorFilesContext);
  if (!ctx) throw new Error("useEditorFiles must be used within EditorFilesProvider");
  return ctx;
}
