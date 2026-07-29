import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({ openContextTab: vi.fn() }),
}));

const H = vi.hoisted(() => ({
  GetMyBoard: vi.fn(),
  GetVCSAuth: vi.fn(),
  GetPendingCodeReviews: vi.fn(),
  DismissReviewRequest: vi.fn(),
  RefreshMyAssignedJiraIssues: vi.fn(),
  ListMyAssignedJiraIssues: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL: vi.fn() }));

import MyBoardPage from "./MyBoardPage";

beforeEach(() => {
  vi.clearAllMocks();
  H.GetVCSAuth.mockResolvedValue(null);
  H.GetPendingCodeReviews.mockResolvedValue([]);
  H.GetMyBoard.mockResolvedValue({
    blockers: [],
    myAssignments: [],
    myOpenPRs: [],
    myAssignedJiraIssues: [
      { key: "JH-1", summary: "Fix login", status: "Open", issueType: "Task", priority: "High", created: "2026-07-20T10:00:00Z", seen: false, fetchedAt: "" },
    ],
  });
  H.RefreshMyAssignedJiraIssues.mockResolvedValue([
    { key: "JH-2", summary: "New bug", status: "Open", issueType: "Bug", priority: "Highest", created: "2026-07-27T10:00:00Z", seen: false, fetchedAt: "" },
  ]);
  H.ListMyAssignedJiraIssues.mockResolvedValue([]);
});

describe("MyBoardPage — My Jira To-Do", () => {
  it("renders assigned issues from the board with a 'new' badge", async () => {
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Fix login")).toBeInTheDocument());
    expect(screen.getByText("JH-1")).toBeInTheDocument();
    expect(screen.getByText("new")).toBeInTheDocument();
  });

  it("Refresh re-fetches from Jira and re-renders", async () => {
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Fix login")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /refresh assigned jira/i }));
    await waitFor(() => expect(H.RefreshMyAssignedJiraIssues).toHaveBeenCalledWith("priority"));
    await waitFor(() => expect(screen.getByText("New bug")).toBeInTheDocument());
  });
});
