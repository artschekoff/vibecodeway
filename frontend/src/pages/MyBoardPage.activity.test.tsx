import { render, screen } from "@testing-library/react";
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
  ReconcileMyOpenPRs: vi.fn(),
  ListMyAssignedJiraIssues: vi.fn(),
  ListOrganizations: vi.fn(),
  GetJiraInstanceURL: vi.fn(),
  CreateContextFromJiraIssue: vi.fn(),
  ListTodoNext: vi.fn(),
  GenerateTodoNext: vi.fn(),
  AdjustTodoNext: vi.fn(),
  ReorderTodoNext: vi.fn(),
  DeleteTodoNextItem: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL: vi.fn() }));

import MyBoardPage from "./MyBoardPage";

beforeEach(() => {
  vi.clearAllMocks();
  H.GetVCSAuth.mockResolvedValue(null);
  H.GetPendingCodeReviews.mockResolvedValue([]);
  H.ListTodoNext.mockResolvedValue([]);
  H.GetMyBoard.mockResolvedValue({
    blockers: [],
    myAssignments: [],
    myOpenPRs: [],
    myAssignedJiraIssues: [],
    recentActivity: [{
      eventId: "e1", contextId: "c1", contextKey: "JH-4922", type: "pr_review_changed",
      sourceType: "github", author: "Aleisha", summary: "PR #41 approved by Aleisha",
      createdAt: new Date().toISOString(),
    }],
  });
  H.RefreshMyAssignedJiraIssues.mockResolvedValue([]);
  H.ListMyAssignedJiraIssues.mockResolvedValue([]);
  H.ListOrganizations.mockResolvedValue([{ id: "o1", name: "Acme" }]);
  H.GetJiraInstanceURL.mockResolvedValue("https://jira.example.com");
  H.CreateContextFromJiraIssue.mockResolvedValue({ id: "c9", key: "JH-1", orgId: "o1" });
});

describe("MyBoardPage — Recent Activity", () => {
  it("renders recent activity events from the board", async () => {
    render(<MyBoardPage />);
    expect(await screen.findByText("PR #41 approved by Aleisha")).toBeInTheDocument();
    expect(screen.getByText("JH-4922")).toBeInTheDocument();
  });
});
