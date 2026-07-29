import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const mocks = vi.hoisted(() => ({
  forceCloseTab: vi.fn(), closeAllTabs: vi.fn(), pinTab: vi.fn(), unpinTab: vi.fn(),
  setActiveTab: vi.fn(), closeTab: vi.fn(), reorderTabs: vi.fn(),
}));
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({
    tabs: [{ id: "t1", type: "context", title: "sprint", contextKey: "sprint", contextId: "c1", pinned: false }],
    activeTabId: "t1",
    ...mocks,
  }),
}));
vi.mock("../hooks/useContextBadges", () => ({ useContextBadges: () => new Map() }));

import TabBar from "./TabBar";

beforeEach(() => vi.clearAllMocks());

function openMenu() {
  render(<TabBar />);
  fireEvent.contextMenu(screen.getByText("sprint"));
}

describe("TabBar right-click menu", () => {
  it("Close calls forceCloseTab for the tab", () => {
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Close" }));
    expect(mocks.forceCloseTab).toHaveBeenCalledWith("t1");
  });

  it("Close all calls closeAllTabs", () => {
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Close all" }));
    expect(mocks.closeAllTabs).toHaveBeenCalled();
  });

  it("Pin calls pinTab for an unpinned tab", () => {
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Pin" }));
    expect(mocks.pinTab).toHaveBeenCalledWith("t1");
  });
});
