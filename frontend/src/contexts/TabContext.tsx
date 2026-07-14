import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { arrayMove } from "@dnd-kit/sortable";
import type { Tab } from "../types";

const TAB_STORAGE_KEY = "context-consolidator:tabs-v1";

interface TabContextValue {
  tabs: Tab[];
  activeTabId: string | null;
  openOrgTab: (orgId: string, orgName: string) => void;
  openContextTab: (contextId: string, contextKey: string, orgName?: string, orgId?: string) => void;
  openNewContextTab: (contextId: string, contextKey: string, orgName?: string, orgId?: string) => void;
  openSettingsTab: () => void;
  openLogsTab: () => void;
  openPromptsTab: () => void;
  openBoardTab: () => void;
  openAskTab: () => void;
  openTimelineTab: () => void;
  closeTab: (tabId: string) => void;
  forceCloseTab: (tabId: string) => void;
  pinTab: (tabId: string) => void;
  unpinTab: (tabId: string) => void;
  setActiveTab: (tabId: string) => void;
  updateTabTitle: (tabId: string, title: string) => void;
  navigateBack: (tabId: string) => void;
  reorderTabs: (activeId: string, overId: string) => void;
}

const TabContext = createContext<TabContextValue | null>(null);

export function useTabContext(): TabContextValue {
  const ctx = useContext(TabContext);
  if (!ctx) throw new Error("useTabContext must be used inside TabProvider");
  return ctx;
}

let _idCounter = 0;
function newTabId(): string {
  return `tab-${++_idCounter}`;
}

function isValidTab(x: unknown): x is Tab {
  if (typeof x !== "object" || x === null) return false;
  const t = x as Record<string, unknown>;
  return typeof t.id === "string" && typeof t.type === "string" && typeof t.title === "string";
}

function readTabStorage(): { tabs: Tab[]; activeTabId: string | null } {
  try {
    const raw = localStorage.getItem(TAB_STORAGE_KEY);
    if (!raw) return { tabs: [], activeTabId: null };
    const parsed = JSON.parse(raw) as { tabs?: unknown; activeTabId?: unknown };
    const tabs = Array.isArray(parsed.tabs) ? (parsed.tabs as unknown[]).filter(isValidTab) : [];
    for (const t of tabs) {
      const n = parseInt((t.id ?? "").replace("tab-", ""), 10);
      if (!isNaN(n) && n > _idCounter) _idCounter = n;
    }
    const activeTabIdRaw = typeof parsed.activeTabId === "string" ? parsed.activeTabId : null;
    const activeTabId = activeTabIdRaw && tabs.some((t) => t.id === activeTabIdRaw)
      ? activeTabIdRaw
      : (tabs[0]?.id ?? null);
    return { tabs, activeTabId };
  } catch {
    return { tabs: [], activeTabId: null };
  }
}

export function TabProvider({ children }: { children: ReactNode }) {
  const _initial = readTabStorage();
  const [tabs, setTabs] = useState<Tab[]>(_initial.tabs);
  const [activeTabId, setActiveTabId] = useState<string | null>(_initial.activeTabId);

  const previousActiveTabIdRef = useRef<string | null>(null);
  const previousActiveTabIdTrackerRef = useRef<string | null>(null);

  useEffect(() => {
    previousActiveTabIdRef.current = previousActiveTabIdTrackerRef.current;
    previousActiveTabIdTrackerRef.current = activeTabId;
  }, [activeTabId]);

  useEffect(() => {
    try {
      localStorage.setItem(TAB_STORAGE_KEY, JSON.stringify({ tabs, activeTabId }));
    } catch {
      // localStorage full or unavailable — best-effort
    }
  }, [tabs, activeTabId]);

  const focusOrOpen = useCallback(
    (
      predicate: (t: Tab) => boolean,
      buildTab: () => Tab
    ) => {
      setTabs((prev) => {
        const existing = prev.find(predicate);
        if (existing) {
          setActiveTabId(existing.id);
          return prev;
        }
        const tab = buildTab();
        setActiveTabId(tab.id);
        return [...prev, tab];
      });
    },
    []
  );

  const openOrgTab = useCallback(
    (orgId: string, orgName: string) => {
      focusOrOpen(
        (t) => t.type === "org" && t.orgId === orgId,
        () => ({ id: newTabId(), type: "org" as const, orgId, title: orgName, pinned: false })
      );
    },
    [focusOrOpen]
  );

  const openContextTab = useCallback(
    (contextId: string, contextKey: string, orgName?: string, orgId?: string) => {
      focusOrOpen(
        (t) => t.type === "context" && t.contextId === contextId,
        () => ({
          id: newTabId(),
          type: "context" as const,
          contextId,
          contextKey,
          orgId,
          orgName,
          title: contextKey,
          pinned: false,
        })
      );
    },
    [focusOrOpen]
  );

  const openNewContextTab = useCallback(
    (contextId: string, contextKey: string, orgName?: string, orgId?: string) => {
      setTabs((prev) => {
        const tab: Tab = {
          id: newTabId(),
          type: "context" as const,
          contextId,
          contextKey,
          orgId,
          orgName,
          title: contextKey,
          pinned: false,
        };
        setActiveTabId(tab.id);
        return [...prev, tab];
      });
    },
    []
  );

  const openSettingsTab = useCallback(() => {
    focusOrOpen(
      (t) => t.type === "settings",
      () => ({ id: newTabId(), type: "settings" as const, title: "Settings", pinned: false })
    );
  }, [focusOrOpen]);

  const openLogsTab = useCallback(() => {
    focusOrOpen(
      (t) => t.type === "logs",
      () => ({ id: newTabId(), type: "logs" as const, title: "App Logs", pinned: false })
    );
  }, [focusOrOpen]);

  const openPromptsTab = useCallback(() => {
    focusOrOpen(
      (t) => t.type === "prompts",
      () => ({ id: newTabId(), type: "prompts" as const, title: "AI Prompts", pinned: false })
    );
  }, [focusOrOpen]);

  const openBoardTab = useCallback(() => {
    focusOrOpen(
      (t) => t.type === "board",
      () => ({ id: newTabId(), type: "board" as const, title: "My Board", pinned: false })
    );
  }, [focusOrOpen]);

  const openAskTab = useCallback(() => {
    focusOrOpen(
      (t) => t.type === "ask",
      () => ({ id: newTabId(), type: "ask" as const, title: "Ask AI", pinned: false })
    );
  }, [focusOrOpen]);

  const openTimelineTab = useCallback(() => {
    focusOrOpen(
      (t) => t.type === "timeline",
      () => ({ id: newTabId(), type: "timeline" as const, title: "Activity Timeline", pinned: false })
    );
  }, [focusOrOpen]);

  const closeTab = useCallback((tabId: string) => {
    setTabs((prev) => {
      const idx = prev.findIndex((t) => t.id === tabId);
      if (idx === -1) return prev;
      const tab = prev[idx];
      if (tab.pinned) return prev; // pinned tabs cannot be closed
      const next = prev.filter((t) => t.id !== tabId);
      setActiveTabId((active) => {
        if (active !== tabId) return active;
        const neighbour = next[idx] ?? next[idx - 1] ?? null;
        return neighbour?.id ?? null;
      });
      return next;
    });
  }, []);

  const forceCloseTab = useCallback((tabId: string) => {
    setTabs((prev) => {
      const idx = prev.findIndex((t) => t.id === tabId);
      if (idx === -1) return prev;
      const next = prev.filter((t) => t.id !== tabId);
      setActiveTabId((active) => {
        if (active !== tabId) return active;
        const neighbour = next[idx] ?? next[idx - 1] ?? null;
        return neighbour?.id ?? null;
      });
      return next;
    });
  }, []);

  const pinTab = useCallback((tabId: string) => {
    setTabs((prev) =>
      prev.map((t) => (t.id === tabId ? { ...t, pinned: true } : t))
    );
  }, []);

  const unpinTab = useCallback((tabId: string) => {
    setTabs((prev) =>
      prev.map((t) => (t.id === tabId ? { ...t, pinned: false } : t))
    );
  }, []);

  const setActiveTab = useCallback((tabId: string) => {
    setActiveTabId(tabId);
  }, []);

  const updateTabTitle = useCallback((tabId: string, title: string) => {
    setTabs((prev) =>
      prev.map((t) => (t.id === tabId ? { ...t, title } : t))
    );
  }, []);

  const reorderTabs = useCallback((activeId: string, overId: string) => {
    setTabs((prev) => {
      const from = prev.findIndex((t) => t.id === activeId);
      const to = prev.findIndex((t) => t.id === overId);
      if (from === -1 || to === -1) return prev;
      return arrayMove(prev, from, to);
    });
  }, []);

  const navigateBack = useCallback(
    (tabId: string) => {
      setTabs((prev) => {
        const tab = prev.find((t) => t.id === tabId);
        if (!tab) return prev;

        if (tab.type === "context") {
          const prevId = previousActiveTabIdRef.current;
          const prevTab = prevId ? prev.find((t) => t.id === prevId && t.id !== tabId) : null;
          if (prevTab) {
            setActiveTabId(prevTab.id);
          } else if (tab.orgId) {
            const orgTab = prev.find((t) => t.type === "org" && t.orgId === tab.orgId);
            if (orgTab) {
              setActiveTabId(orgTab.id);
            } else {
              const name = tab.orgName ?? tab.orgId;
              const newOrgTab: Tab = {
                id: newTabId(),
                type: "org" as const,
                orgId: tab.orgId,
                orgName: tab.orgName,
                title: name,
                pinned: false,
              };
              setActiveTabId(newOrgTab.id);
              return [...prev, newOrgTab];
            }
          }
        } else if (tab.type === "settings") {
          const prevId = previousActiveTabIdRef.current;
          const prevTab = prevId ? prev.find((t) => t.id === prevId) : null;
          if (prevTab) {
            setActiveTabId(prevTab.id);
          } else {
            const fallback = prev.find((t) => t.type !== "settings");
            if (fallback) setActiveTabId(fallback.id);
          }
        }

        return prev;
      });
    },
    []
  );

  return (
    <TabContext.Provider
      value={{
        tabs,
        activeTabId,
        openOrgTab,
        openContextTab,
        openNewContextTab,
        openSettingsTab,
        openLogsTab,
        openPromptsTab,
        openBoardTab,
        openAskTab,
        openTimelineTab,
        closeTab,
        forceCloseTab,
        pinTab,
        unpinTab,
        setActiveTab,
        updateTabTitle,
        navigateBack,
        reorderTabs,
      }}
    >
      {children}
    </TabContext.Provider>
  );
}
