import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { TabProvider, useTabContext } from "./TabContext";
import type { ReactNode } from "react";

const STORAGE_KEY = "vibecodeway:tabs-v1";

function wrapper({ children }: { children: ReactNode }) {
  return <TabProvider>{children}</TabProvider>;
}

// Node 25 exposes its own native `localStorage` global that lacks `.clear()`,
// shadowing jsdom's implementation. We replace it with a proper in-memory mock.
function makeLocalStorageMock(): Storage {
  let store: Record<string, string> = {};
  return {
    get length() { return Object.keys(store).length; },
    key(index: number) { return Object.keys(store)[index] ?? null; },
    getItem(k: string) { return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null; },
    setItem(k: string, v: string) { store[k] = String(v); },
    removeItem(k: string) { delete store[k]; },
    clear() { store = {}; },
  } as Storage;
}

const localStorageMock = makeLocalStorageMock();

beforeEach(() => {
  vi.stubGlobal("localStorage", localStorageMock);
  localStorageMock.clear();
});

afterEach(() => {
  localStorageMock.clear();
  vi.unstubAllGlobals();
});

describe("tab persistence", () => {
  it("starts with empty tabs when localStorage is empty", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    expect(result.current.tabs).toEqual([]);
    expect(result.current.activeTabId).toBeNull();
  });

  it("writes tabs to localStorage when a tab is opened", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });

    act(() => result.current.openSettingsTab());

    const stored = JSON.parse(localStorageMock.getItem(STORAGE_KEY)!);
    expect(stored.tabs).toHaveLength(1);
    expect(stored.tabs[0].type).toBe("settings");
    expect(stored.activeTabId).toBe(stored.tabs[0].id);
  });

  it("restores tabs from localStorage on mount", () => {
    const storedTabs = [
      { id: "tab-1", type: "settings", title: "Settings", pinned: false },
    ];
    localStorageMock.setItem(STORAGE_KEY, JSON.stringify({ tabs: storedTabs, activeTabId: "tab-1" }));

    const { result } = renderHook(() => useTabContext(), { wrapper });
    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].type).toBe("settings");
    expect(result.current.activeTabId).toBe("tab-1");
  });

  it("new tab IDs do not collide with restored IDs", () => {
    const storedTabs = [
      { id: "tab-5", type: "settings", title: "Settings", pinned: false },
    ];
    localStorageMock.setItem(STORAGE_KEY, JSON.stringify({ tabs: storedTabs, activeTabId: "tab-5" }));

    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openOrgTab("org-1", "Acme"));

    const ids = result.current.tabs.map((t) => t.id);
    expect(new Set(ids).size).toBe(ids.length); // no duplicates
    // The new tab must be higher than tab-5
    const newTab = result.current.tabs.find((t) => t.type === "org");
    const newNum = parseInt(newTab!.id.replace("tab-", ""), 10);
    expect(newNum).toBeGreaterThan(5);
  });

  it("removes tab from localStorage when closed", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openSettingsTab());
    const tabId = result.current.tabs[0].id;

    act(() => result.current.closeTab(tabId));

    const stored = JSON.parse(localStorageMock.getItem(STORAGE_KEY)!);
    expect(stored.tabs).toHaveLength(0);
  });

  it("falls back to first tab when restored activeTabId does not exist in tab list", () => {
    const storedTabs = [
      { id: "tab-1", type: "settings", title: "Settings", pinned: false },
    ];
    // activeTabId points to a non-existent tab
    localStorageMock.setItem(STORAGE_KEY, JSON.stringify({ tabs: storedTabs, activeTabId: "tab-999" }));

    const { result } = renderHook(() => useTabContext(), { wrapper });
    expect(result.current.activeTabId).toBe("tab-1");
  });

  it("ignores corrupt tabs entries that fail the shape guard", () => {
    const mixed = [
      { id: "tab-1", type: "settings", title: "Settings", pinned: false }, // valid
      { notATab: true }, // invalid - missing id, type, title
      null, // invalid
    ];
    localStorageMock.setItem(STORAGE_KEY, JSON.stringify({ tabs: mixed, activeTabId: "tab-1" }));

    const { result } = renderHook(() => useTabContext(), { wrapper });
    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].id).toBe("tab-1");
    expect(result.current.activeTabId).toBe("tab-1");
  });
});

describe("openTimelineTab", () => {
  it("openTimelineTab opens and focuses a single timeline tab", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openTimelineTab());
    const timelineTabs = result.current.tabs.filter((t) => t.type === "timeline");
    expect(timelineTabs).toHaveLength(1);
    expect(result.current.activeTabId).toBe(timelineTabs[0].id);
    // opening again focuses the existing one, does not duplicate
    act(() => result.current.openTimelineTab());
    expect(result.current.tabs.filter((t) => t.type === "timeline")).toHaveLength(1);
  });
});

describe("openActivityFeedTab", () => {
  it("opens and focuses a single activity-feed tab", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openActivityFeedTab());
    const feedTabs = result.current.tabs.filter((t) => t.type === "activity-feed");
    expect(feedTabs).toHaveLength(1);
    expect(feedTabs[0].title).toBe("Activity Feed");
    expect(result.current.activeTabId).toBe(feedTabs[0].id);
    // opening again focuses the existing one, does not duplicate
    act(() => result.current.openActivityFeedTab());
    expect(result.current.tabs.filter((t) => t.type === "activity-feed")).toHaveLength(1);
  });
});

describe("openProductivityTab", () => {
  it("opens and focuses a single productivity tab", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openProductivityTab());
    expect(result.current.tabs.filter((t) => t.type === "productivity")).toHaveLength(1);
    expect(result.current.tabs.find((t) => t.type === "productivity")?.title).toBe("Closed Tasks");

    act(() => result.current.openProductivityTab());
    expect(result.current.tabs.filter((t) => t.type === "productivity")).toHaveLength(1);
  });
});

describe("openAnalyticsTab", () => {
  it("opens and focuses a single analytics tab", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openAnalyticsTab());
    expect(result.current.tabs.filter((t) => t.type === "analytics")).toHaveLength(1);
    expect(result.current.tabs.find((t) => t.type === "analytics")?.title).toBe("Analytics");

    act(() => result.current.openAnalyticsTab());
    expect(result.current.tabs.filter((t) => t.type === "analytics")).toHaveLength(1);
  });
});

describe("forceCloseTab", () => {
  it("closes a pinned tab that closeTab would ignore", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });

    act(() => result.current.openSettingsTab());
    const tabId = result.current.tabs[0].id;
    act(() => result.current.pinTab(tabId));

    // Normal closeTab should leave pinned tab open
    act(() => result.current.closeTab(tabId));
    expect(result.current.tabs).toHaveLength(1);

    // forceCloseTab should remove it
    act(() => result.current.forceCloseTab(tabId));
    expect(result.current.tabs).toHaveLength(0);
  });

  it("is a no-op for a tab ID that does not exist", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openSettingsTab());
    act(() => result.current.forceCloseTab("nonexistent-id"));
    expect(result.current.tabs).toHaveLength(1); // unchanged
  });

  it("updates activeTabId to neighbour when active tab is force-closed", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openSettingsTab());
    act(() => result.current.openOrgTab("org-1", "Acme"));
    const activeId = result.current.activeTabId!;
    const otherTab = result.current.tabs.find((t) => t.id !== activeId)!;

    act(() => result.current.forceCloseTab(activeId));

    expect(result.current.activeTabId).toBe(otherTab.id);
    expect(result.current.tabs).toHaveLength(1);
  });

  it("sets activeTabId to null when the only tab is force-closed", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openSettingsTab());
    const tabId = result.current.tabs[0].id;

    act(() => result.current.forceCloseTab(tabId));

    expect(result.current.tabs).toHaveLength(0);
    expect(result.current.activeTabId).toBeNull();
  });

  it("sets activeTabId to successor when a non-last tab is force-closed", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper });
    act(() => result.current.openSettingsTab());
    act(() => result.current.openOrgTab("org-1", "Acme"));
    // Force-close the first tab (settings), which is not the active tab
    const firstTabId = result.current.tabs[0].id;
    const secondTabId = result.current.tabs[1].id;
    // Make first tab active so closing it forces successor selection
    act(() => result.current.setActiveTab(firstTabId));

    act(() => result.current.forceCloseTab(firstTabId));

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.activeTabId).toBe(secondTabId);
  });
});
