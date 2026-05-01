import {
  createContext,
  useCallback,
  useContext,
  useState,
  type ReactNode,
} from "react";
import type { Tab } from "../types";

interface TabContextValue {
  tabs: Tab[];
  activeTabId: string | null;
  openOrgTab: (orgId: string, orgName: string) => void;
  openContextTab: (contextId: string, contextKey: string) => void;
  openNewContextTab: (contextId: string, contextKey: string) => void;
  openSettingsTab: () => void;
  closeTab: (tabId: string) => void;
  pinTab: (tabId: string) => void;
  unpinTab: (tabId: string) => void;
  setActiveTab: (tabId: string) => void;
  updateTabTitle: (tabId: string, title: string) => void;
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

export function TabProvider({ children }: { children: ReactNode }) {
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);

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
    (contextId: string, contextKey: string) => {
      focusOrOpen(
        (t) => t.type === "context" && t.contextId === contextId,
        () => ({
          id: newTabId(),
          type: "context" as const,
          contextId,
          title: contextKey,
          pinned: false,
        })
      );
    },
    [focusOrOpen]
  );

  const openNewContextTab = useCallback(
    (contextId: string, contextKey: string) => {
      setTabs((prev) => {
        const tab: Tab = {
          id: newTabId(),
          type: "context" as const,
          contextId,
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

  return (
    <TabContext.Provider
      value={{
        tabs,
        activeTabId,
        openOrgTab,
        openContextTab,
        openNewContextTab,
        openSettingsTab,
        closeTab,
        pinTab,
        unpinTab,
        setActiveTab,
        updateTabTitle,
      }}
    >
      {children}
    </TabContext.Provider>
  );
}
