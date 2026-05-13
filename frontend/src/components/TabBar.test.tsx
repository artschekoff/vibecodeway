import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const mockClose = vi.fn();
const mockSetActive = vi.fn();
const mockPin = vi.fn();
const mockUnpin = vi.fn();

vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({
    tabs: [
      { id: "t1", type: "context", title: "sprint", contextKey: "sprint", pinned: false },
    ],
    activeTabId: "t1",
    closeTab: mockClose,
    setActiveTab: mockSetActive,
    pinTab: mockPin,
    unpinTab: mockUnpin,
  }),
}));

import TabBar from "./TabBar";

beforeEach(() => {
  mockClose.mockClear();
  mockSetActive.mockClear();
});

describe("TabBar", () => {
  it("renders a tab-item__close-zone wrapper around the close button", () => {
    const { container } = render(<TabBar />);
    const zone = container.querySelector(".tab-item__close-zone");
    expect(zone).not.toBeNull();
    const btn = zone!.querySelector("button");
    expect(btn).not.toBeNull();
  });

  it("close button is inside close-zone, not directly in tab-item", () => {
    const { container } = render(<TabBar />);
    const tab = container.querySelector(".tab-item")!;
    const directButtons = Array.from(tab.children).filter((el) => el.tagName === "BUTTON");
    expect(directButtons).toHaveLength(0);
  });

  it("clicking the close button calls closeTab with the tab id", async () => {
    const user = userEvent.setup();
    const { container } = render(<TabBar />);
    const btn = container.querySelector(".tab-item__close-zone button")!;
    await user.click(btn);
    expect(mockClose).toHaveBeenCalledWith("t1");
  });

  it("clicking the tab title calls setActiveTab, not closeTab", async () => {
    const user = userEvent.setup();
    render(<TabBar />);
    const title = screen.getByText("sprint");
    await user.click(title);
    expect(mockSetActive).toHaveBeenCalledWith("t1");
    expect(mockClose).not.toHaveBeenCalled();
  });
});
