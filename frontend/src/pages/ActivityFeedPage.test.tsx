import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { openContextTab } = vi.hoisted(() => ({ openContextTab: vi.fn() }));
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({ openContextTab }),
}));

const { GetActivityFeed } = vi.hoisted(() => ({ GetActivityFeed: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({ GetActivityFeed }));

import ActivityFeedPage from "./ActivityFeedPage";

const items = [
  { id: "n1", contextId: "c1", contextKey: "PROJ-1", entityType: "pull_request", eventType: "pr_merged", oldValue: "", newValue: "merged", createdAt: "2026-07-20T10:00:00Z" },
  { id: "n2", contextId: "c2", contextKey: "PROJ-2", entityType: "jira_issue", eventType: "jira_status_changed", oldValue: "To Do", newValue: "In Progress", createdAt: "2026-07-20T09:00:00Z" },
];

beforeEach(() => {
  openContextTab.mockClear();
  GetActivityFeed.mockReset();
});

describe("ActivityFeedPage", () => {
  it("renders items in received order and navigates to the context on click", async () => {
    GetActivityFeed.mockResolvedValue(items);
    const user = userEvent.setup();
    render(<ActivityFeedPage />);

    // Wait for load
    await screen.findByText("PR merged");
    expect(screen.getByText(/Jira status: To Do → In Progress/)).toBeInTheDocument();

    // Preserves backend order (newest first)
    const descs = screen.getAllByTestId("activity-desc").map((e) => e.textContent);
    expect(descs[0]).toBe("PR merged");

    await user.click(screen.getByText("PR merged"));
    expect(openContextTab).toHaveBeenCalledWith("c1", "PROJ-1");
  });

  it("shows an empty state when there is no activity", async () => {
    GetActivityFeed.mockResolvedValue([]);
    render(<ActivityFeedPage />);
    expect(await screen.findByText("No activity yet.")).toBeInTheDocument();
  });
});
