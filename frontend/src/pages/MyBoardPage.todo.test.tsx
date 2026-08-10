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
  ReconcileMyOpenPRs: vi.fn(),
  CreateContextFromJiraIssue: vi.fn(),
  ListOrganizations: vi.fn(),
  GetJiraInstanceURL: vi.fn(),
  ListTodoNext: vi.fn(),
  GenerateTodoNext: vi.fn(),
  AdjustTodoNext: vi.fn(),
  ReorderTodoNext: vi.fn(),
  DeleteTodoNextItem: vi.fn(),
  DraftTodoMessage: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL: vi.fn() }));

import MyBoardPage from "./MyBoardPage";

beforeEach(() => {
  vi.clearAllMocks();
  H.GetVCSAuth.mockResolvedValue(null);
  H.GetPendingCodeReviews.mockResolvedValue([]);
  H.GetMyBoard.mockResolvedValue({ blockers: [], myAssignments: [], myOpenPRs: [], myAssignedJiraIssues: [], recentActivity: [] });
  H.ListOrganizations.mockResolvedValue([]);
  H.GetJiraInstanceURL.mockResolvedValue("");
  H.ListTodoNext.mockResolvedValue([
    { id: "1", kind: "rerequest_pr", title: "Ping reviewers on #42", priority: "high", position: 0, refKey: "o/r#42", status: "pending", source: "ai" },
  ]);
  H.GenerateTodoNext.mockResolvedValue([
    { id: "2", kind: "implement", title: "Finish JH-1", priority: "critical", position: 0, status: "pending", source: "ai" },
  ]);
  H.AdjustTodoNext.mockResolvedValue([
    { id: "3", kind: "check_status", title: "Check staging deploy", priority: "high", position: 0, status: "pending", source: "ai" },
  ]);
});

describe("MyBoardPage — TODO NEXT (BY AI)", () => {
  it("renders the persisted list on load", async () => {
    render(<MyBoardPage />);
    expect(await screen.findByText(/Ping reviewers on #42/)).toBeInTheDocument();
  });

  it("Generate replaces the list", async () => {
    render(<MyBoardPage />);
    await screen.findByText(/Ping reviewers on #42/);
    fireEvent.click(screen.getByRole("button", { name: /Generate/i }));
    await waitFor(() => expect(screen.getByText(/Finish JH-1/)).toBeInTheDocument());
  });

  it("Adjust applies a comment and revises the list", async () => {
    render(<MyBoardPage />);
    await screen.findByText(/Ping reviewers on #42/);
    fireEvent.click(screen.getByRole("button", { name: /Adjust/i }));
    fireEvent.change(screen.getByPlaceholderText(/drop the docs task/i), { target: { value: "do the deploy check first" } });
    fireEvent.click(screen.getByRole("button", { name: /Apply adjustment/i }));
    await waitFor(() => expect(H.AdjustTodoNext).toHaveBeenCalledWith("do the deploy check first"));
    await waitFor(() => expect(screen.getByText(/Check staging deploy/)).toBeInTheDocument());
  });

  it("shows an Open context button that opens the PR URL", async () => {
    const { BrowserOpenURL } = await import("../../wailsjs/runtime/runtime");
    H.ListTodoNext.mockResolvedValue([
      { id: "1", kind: "rerequest_pr", title: "Ping reviewers on #42", priority: "high", position: 0, refType: "pr", refKey: "o/r#42", status: "pending", source: "ai" },
    ]);
    render(<MyBoardPage />);
    const open = await screen.findByRole("button", { name: /Open context/i });
    fireEvent.click(open);
    expect(BrowserOpenURL).toHaveBeenCalledWith("https://github.com/o/r/pull/42");
  });

  it("shows no Open context button when the todo has no resolvable ref", async () => {
    H.ListTodoNext.mockResolvedValue([
      { id: "1", kind: "implement", title: "Finish the thing", priority: "high", position: 0, status: "pending", source: "ai" },
    ]);
    render(<MyBoardPage />);
    await screen.findByText(/Finish the thing/);
    expect(screen.queryByRole("button", { name: /Open context/i })).toBeNull();
  });

  it("opens the draft modal and shows the AI-drafted message", async () => {
    H.DraftTodoMessage.mockResolvedValue("Please review o/r#42.");
    H.ListTodoNext.mockResolvedValue([
      { id: "1", kind: "rerequest_pr", title: "Ping reviewers on #42", priority: "high", position: 0, refType: "pr", refKey: "o/r#42", rationale: "stale", status: "pending", source: "ai" },
    ]);
    render(<MyBoardPage />);
    fireEvent.click(await screen.findByRole("button", { name: /Draft reviewer nudge/i }));
    expect(await screen.findByDisplayValue("Please review o/r#42.")).toBeInTheDocument();
  });
});
