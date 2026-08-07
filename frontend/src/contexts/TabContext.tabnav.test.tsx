import { describe, it, expect, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { TabProvider, useTabContext } from "./TabContext";

describe("TabContext tab navigation", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("focusNextTab / focusPrevTab cycle open tabs with wrap", () => {
    const { result } = renderHook(() => useTabContext(), { wrapper: TabProvider });
    act(() => {
      result.current.openContextTab("c1", "A");
      result.current.openContextTab("c2", "B");
      result.current.openContextTab("c3", "C");
    });
    act(() => {
      result.current.setActiveTab(result.current.tabs[0].id);
    });
    act(() => {
      result.current.focusNextTab();
    });
    expect(result.current.activeTabId).toBe(result.current.tabs[1].id);

    act(() => {
      result.current.setActiveTab(result.current.tabs[2].id);
      result.current.focusNextTab();
    });
    expect(result.current.activeTabId).toBe(result.current.tabs[0].id); // wrap forward

    act(() => {
      result.current.setActiveTab(result.current.tabs[0].id);
      result.current.focusPrevTab();
    });
    expect(result.current.activeTabId).toBe(result.current.tabs[2].id); // wrap back
  });
});
