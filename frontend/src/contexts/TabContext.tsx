import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import type { Tab } from "../types";

const TAB_STORAGE_KEY = "context-consolidator:tabs-v1";

interface TabContextValue {
  tabs: Tab[];
  activeTabId: string | null;
  openOrgTab: (orgId: string, orgName: string) => void;
  openContextTab: (contextId: string, contextKey: string, orgName?: string) => void;
  openNewContextTab: (contextId: string, contextKey: string, orgName?: string) => void;
  openSettingsTab: () => void;
  closeTab: (tabId: string) => void;
  forceCloseTab: (tabId: string) => void;
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
    (contextId: string, contextKey: string, orgName?: string) => {
      const title = orgName ? `${orgName} / ${contextKey}` : contextKey;
      focusOrOpen(
        (t) => t.type === "context" && t.contextId === contextId,
        () => ({
          id: newTabId(),
          type: "context" as const,
          contextId,
          title,
          pinned: false,
        })
      );
    },
    [focusOrOpen]
  );

  const openNewContextTab = useCallback(
    (contextId: string, contextKey: string, orgName?: string) => {
      const title = orgName ? `${orgName} / ${contextKey}` : contextKey;
      setTabs((prev) => {
        const tab: Tab = {
          id: newTabId(),
          type: "context" as const,
          contextId,
          title,
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
        forceCloseTab,
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
