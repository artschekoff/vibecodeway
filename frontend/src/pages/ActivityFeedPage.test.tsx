import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { openContextTab } = vi.hoisted(() => ({ openContextTab: vi.fn() }));
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({ openContextTab }),
}));

const { GetActivityFeed, SearchContextIDs, ListMyAssignedJiraIssues, MarkMyAssignedJiraIssuesSeen } = vi.hoisted(() => ({
  GetActivityFeed: vi.fn(),
  SearchContextIDs: vi.fn(),
  ListMyAssignedJiraIssues: vi.fn().mockResolvedValue([]),
  MarkMyAssignedJiraIssuesSeen: vi.fn().mockResolvedValue(undefined),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetActivityFeed, SearchContextIDs, ListMyAssignedJiraIssues, MarkMyAssignedJiraIssuesSeen }));

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

  it("shows the note attached by a notification rule", async () => {
    GetActivityFeed.mockResolvedValue([
      {
        id: "n1", contextId: "c1", contextKey: "PROJ-1", entityType: "jira_issue",
        eventType: "jira_status_changed", oldValue: "In Review", newValue: "Done",
        note: "tell QA we are ready", createdAt: "2026-08-31T10:00:00Z",
      },
    ]);

    render(<ActivityFeedPage />);

    expect(await screen.findByTestId("activity-note")).toHaveTextContent("tell QA we are ready");
  });

  it("renders no note element when the event carried none", async () => {
    GetActivityFeed.mockResolvedValue([
      {
        id: "n2", contextId: "c1", contextKey: "PROJ-1", entityType: "jira_issue",
        eventType: "jira_status_changed", oldValue: "To Do", newValue: "In Review",
        note: "", createdAt: "2026-08-31T10:00:00Z",
      },
    ]);

    render(<ActivityFeedPage />);

    await screen.findByTestId("activity-desc");
    expect(screen.queryByTestId("activity-note")).toBeNull();
  });
});
