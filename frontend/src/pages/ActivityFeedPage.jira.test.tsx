import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const H = vi.hoisted(() => ({
  GetActivityFeed: vi.fn(),
  SearchContextIDs: vi.fn(),
  ListMyAssignedJiraIssues: vi.fn(),
  MarkMyAssignedJiraIssuesSeen: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../contexts/TabContext", () => ({ useTabContext: () => ({ openContextTab: vi.fn() }) }));

import ActivityFeedPage from "./ActivityFeedPage";

beforeEach(() => {
  vi.clearAllMocks();
  H.GetActivityFeed.mockResolvedValue([]);
  H.SearchContextIDs.mockResolvedValue([]);
  H.ListMyAssignedJiraIssues.mockResolvedValue([
    { key: "JH-9", summary: "Assigned to me", status: "Open", issueType: "Task", priority: "High", created: "2026-07-26T10:00:00Z", seen: false, fetchedAt: "" },
  ]);
  H.MarkMyAssignedJiraIssuesSeen.mockResolvedValue(undefined);
});

describe("ActivityFeedPage — assigned Jira", () => {
  it("shows unseen assigned issues with a 'new' badge and marks them seen", async () => {
    render(<ActivityFeedPage />);
    await waitFor(() => expect(screen.getByText("Assigned to me")).toBeInTheDocument());
    expect(screen.getByText("JH-9")).toBeInTheDocument();
    expect(screen.getByText("new")).toBeInTheDocument();
    await waitFor(() => expect(H.MarkMyAssignedJiraIssuesSeen).toHaveBeenCalled());
  });
});
