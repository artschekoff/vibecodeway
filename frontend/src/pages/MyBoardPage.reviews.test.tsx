import { render, screen, waitFor } from "@testing-library/react";
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
vi.mock("../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL: vi.fn(), EventsOn: vi.fn(() => () => {}) }));

import MyBoardPage from "./MyBoardPage";

beforeEach(() => {
  vi.clearAllMocks();
  H.GetVCSAuth.mockResolvedValue(null);
  H.ListTodoNext.mockResolvedValue([]);
  H.ListMyAssignedJiraIssues.mockResolvedValue([]);
  H.ListOrganizations.mockResolvedValue([]);
  H.GetJiraInstanceURL.mockResolvedValue("");
  H.RefreshMyAssignedJiraIssues.mockResolvedValue([]);
  H.GetMyBoard.mockResolvedValue({
    blockers: [], myAssignments: [], myOpenPRs: [], myAssignedJiraIssues: [],
  });
  H.GetPendingCodeReviews.mockResolvedValue([
    { id: "a", repoId: "r1", prNumber: 1, prTitle: "Fresh one", prAuthor: "carol",
      prUrl: "https://x/1", reviewStatus: "review_required", requestedAt: "", fetchedAt: "",
      kind: "requested" },
    { id: "c", repoId: "r1", prNumber: 3, prTitle: "All good", prAuthor: "carol",
      prUrl: "https://x/3", reviewStatus: "approved", requestedAt: "", fetchedAt: "",
      kind: "requested", myReviewState: "approved", reviewedAt: "2026-08-01T00:00:00Z" },
  ]);
});

describe("My Board — PR Reviews stages", () => {
  it("lists reviewed PRs too and badges each with its stage", async () => {
    render(<MyBoardPage />);

    await waitFor(() => expect(screen.getByText("Fresh one")).toBeInTheDocument());
    // Previously hidden because reviewedAt was set.
    expect(screen.getByText("All good")).toBeInTheDocument();
    expect(screen.getByText("Review Required")).toBeInTheDocument();
    expect(screen.getByText("Approved")).toBeInTheDocument();
  });
});
