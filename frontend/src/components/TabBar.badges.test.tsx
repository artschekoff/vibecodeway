import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { useTabContext } = vi.hoisted(() => ({ useTabContext: vi.fn() }));
vi.mock("../contexts/TabContext", () => ({ useTabContext }));

const { useContextBadges } = vi.hoisted(() => ({ useContextBadges: vi.fn() }));
vi.mock("../hooks/useContextBadges", () => ({ useContextBadges }));

import TabBar from "./TabBar"; // TabBar is a default export

beforeEach(() => {
  vi.clearAllMocks();
  useTabContext.mockReturnValue({
    tabs: [{ id: "t1", type: "context", contextId: "c1", contextKey: "den-fix", title: "den-fix", pinned: false }],
    activeTabId: "t1",
    setActiveTab: vi.fn(), closeTab: vi.fn(), pinTab: vi.fn(), unpinTab: vi.fn(), reorderTabs: vi.fn(),
  });
});

describe("TabBar context-plate badges", () => {
  it("shows the new-item count and attention color for a context plate", () => {
    useContextBadges.mockReturnValue(new Map([["c1", { unviewedCount: 5, needsAttention: true }]]));
    const { container } = render(<TabBar />);
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(container.querySelector(".tab-item__icon--attention")).not.toBeNull();
  });

  it("shows no badge when there are no new items and no attention", () => {
    useContextBadges.mockReturnValue(new Map([["c1", { unviewedCount: 0, needsAttention: false }]]));
    const { container } = render(<TabBar />);
    expect(container.querySelector(".tab-item__badge")).toBeNull();
    expect(container.querySelector(".tab-item__icon--attention")).toBeNull();
  });

  it("caps the displayed count at 99+", () => {
    useContextBadges.mockReturnValue(new Map([["c1", { unviewedCount: 150, needsAttention: false }]]));
    render(<TabBar />);
    expect(screen.getByText("99+")).toBeInTheDocument();
  });
});
