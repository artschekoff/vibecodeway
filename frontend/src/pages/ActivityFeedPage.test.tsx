import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { openContextTab } = vi.hoisted(() => ({ openContextTab: vi.fn() }));
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({ openContextTab }),
}));

const { GetActivityFeed, SearchContextIDs } = vi.hoisted(() => ({
  GetActivityFeed: vi.fn(),
  SearchContextIDs: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetActivityFeed, SearchContextIDs }));

import ActivityFeedPage from "./ActivityFeedPage";

const items = [
  { id: "n1", contextId: "c1", contextKey: "PROJ-1", entityType: "pull_request", eventType: "pr_merged", oldValue: "", newValue: "merged", createdAt: "2026-07-20T10:00:00Z" },
  { id: "n2", contextId: "c2", contextKey: "PROJ-2", entityType: "jira_issue", eventType: "jira_status_changed", oldValue: "To Do", newValue: "In Progress", createdAt: "2026-07-20T09:00:00Z" },
];

beforeEach(() => {
  openContextTab.mockClear();
  GetActivityFeed.mockReset();
  SearchContextIDs.mockReset();
  SearchContextIDs.mockResolvedValue([]);
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

  it("re-fetches the feed when Refresh is clicked", async () => {
    GetActivityFeed.mockResolvedValue(items);
    const user = userEvent.setup();
    render(<ActivityFeedPage />);
    await screen.findByText("PR merged");
    expect(GetActivityFeed).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: /refresh/i }));
    expect(GetActivityFeed).toHaveBeenCalledTimes(2);
  });

  it("filters the feed to contexts matching the search (same logic as contexts)", async () => {
    GetActivityFeed.mockResolvedValue(items);
    SearchContextIDs.mockResolvedValue(["c1"]); // only PROJ-1's context matches
    const user = userEvent.setup();
    render(<ActivityFeedPage />);
    await screen.findByText("PR merged");

    await user.type(screen.getByPlaceholderText("Search…"), "referral");

    // Debounced search resolves → PROJ-2 event drops out; PROJ-1 event stays.
    await waitFor(() =>
      expect(screen.queryByText(/Jira status: To Do → In Progress/)).not.toBeInTheDocument(),
    );
    expect(screen.getByText("PR merged")).toBeInTheDocument();
    expect(SearchContextIDs).toHaveBeenCalledWith("referral");
  });
});
