import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({ openContextTab: vi.fn() }),
}));

const H = vi.hoisted(() => ({
  GetMyBoard: vi.fn(), GetVCSAuth: vi.fn(), GetPendingCodeReviews: vi.fn(),
  DismissReviewRequest: vi.fn(), RefreshMyAssignedJiraIssues: vi.fn(),
  ReconcileMyOpenPRs: vi.fn(), ListMyAssignedJiraIssues: vi.fn(),
  ListOrganizations: vi.fn(), GetJiraInstanceURL: vi.fn(),
  CreateContextFromJiraIssue: vi.fn(), ListTodoNext: vi.fn(),
  GenerateTodoNext: vi.fn(), AdjustTodoNext: vi.fn(), ReorderTodoNext: vi.fn(),
  DeleteTodoNextItem: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL: vi.fn(), EventsOn: vi.fn(() => () => {}) }));

import MyBoardPage from "./MyBoardPage";

const issue = (over: Record<string, unknown>) => ({
  key: "JH-1", summary: "s", status: "Open", issueType: "Task",
  priority: "High", created: "2026-07-20T10:00:00Z", seen: true, fetchedAt: "", ...over,
});

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.setItem("cc:board-view", "list");
  H.GetVCSAuth.mockResolvedValue(null);
  H.GetPendingCodeReviews.mockResolvedValue([]);
  H.ListTodoNext.mockResolvedValue([]);
  H.ListMyAssignedJiraIssues.mockResolvedValue([]);
  H.ListOrganizations.mockResolvedValue([{ id: "o1", name: "Acme" }]);
  H.GetJiraInstanceURL.mockResolvedValue("");
  H.GetMyBoard.mockResolvedValue({
    blockers: [], myAssignments: [], myOpenPRs: [],
    myAssignedJiraIssues: [
      issue({ key: "JH-1", summary: "Login crashes", issueType: "Bug" }),
      issue({ key: "JH-2", summary: "Dark mode", issueType: "Story" }),
      issue({ key: "JH-3", summary: "Split the form", issueType: "Subtask" }),
    ],
  });
});

describe("MyBoardPage — issue type", () => {
  it("shows each issue's type as a chip", async () => {
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Login crashes")).toBeInTheDocument());
    // By title, not by text: the type names also appear as filter <option>s.
    expect(screen.getAllByTitle("Issue type").map((el) => el.textContent))
      .toEqual(["Bug", "Story", "Subtask"]);
  });

  it("filters by type, listing the types actually present", async () => {
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Login crashes")).toBeInTheDocument());

    const select = screen.getByLabelText("filter by type") as HTMLSelectElement;
    // Built from the data, not a hardcoded Bug/Feature list — the board carries
    // Subtask/Issue/Story in practice.
    expect([...select.options].map((o) => o.text)).toEqual(["All types", "Bug", "Story", "Subtask"]);

    fireEvent.change(select, { target: { value: "Bug" } });
    expect(screen.getByText("Login crashes")).toBeInTheDocument();
    expect(screen.queryByText("Dark mode")).not.toBeInTheDocument();
    expect(screen.queryByText("Split the form")).not.toBeInTheDocument();
  });

  it("hides the type filter when no issue carries a type", async () => {
    H.GetMyBoard.mockResolvedValue({
      blockers: [], myAssignments: [], myOpenPRs: [],
      myAssignedJiraIssues: [issue({ key: "JH-9", summary: "Untyped", issueType: "" })],
    });
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Untyped")).toBeInTheDocument());
    expect(screen.queryByLabelText("filter by type")).not.toBeInTheDocument();
  });
});
