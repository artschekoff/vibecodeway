import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
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
  H.GetPendingCodeReviews.mockResolvedValue([]);
  H.ListTodoNext.mockResolvedValue([]);
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
  H.ListOrganizations.mockResolvedValue([{ id: "o1", name: "Acme" }]);
  H.GetJiraInstanceURL.mockResolvedValue("https://jira.example.com");
  H.CreateContextFromJiraIssue.mockResolvedValue({ id: "c9", key: "JH-1", orgId: "o1" });
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

  it("offers open/copy links and creates a context from a ticket", async () => {
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Fix login")).toBeInTheDocument());
    // Per-ticket open + copy links (Jira base URL was fetched).
    expect(screen.getByRole("button", { name: /open JH-1 in jira/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /copy JH-1 link/i })).toBeInTheDocument();
    // Create context uses the selected org (single org auto-selected).
    fireEvent.click(screen.getByRole("button", { name: /create context from JH-1/i }));
    await waitFor(() => expect(H.CreateContextFromJiraIssue).toHaveBeenCalledWith("o1", "JH-1"));
  });

  it("filters the list by status", async () => {
    H.GetMyBoard.mockResolvedValue({
      blockers: [], myAssignments: [], myOpenPRs: [],
      myAssignedJiraIssues: [
        { key: "JH-1", summary: "Fix login", status: "To Do", issueType: "Task", priority: "High", created: "", seen: false, fetchedAt: "" },
        { key: "JH-2", summary: "Ship it", status: "Done", issueType: "Task", priority: "Low", created: "", seen: false, fetchedAt: "" },
      ],
    });
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByText("Fix login")).toBeInTheDocument());
    expect(screen.getByText("Ship it")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /filter by status/i }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Done" }));
    await waitFor(() => expect(screen.queryByText("Fix login")).not.toBeInTheDocument());
    expect(screen.getByText("Ship it")).toBeInTheDocument();
    // Multi-select: adding a second status brings the other row back.
    fireEvent.click(screen.getByRole("checkbox", { name: "To Do" }));
    await waitFor(() => expect(screen.getByText("Fix login")).toBeInTheDocument());
    expect(screen.getByText("Ship it")).toBeInTheDocument();
  });

  it("shows an Open-context button when the ticket is already linked", async () => {
    H.GetMyBoard.mockResolvedValue({
      blockers: [], myAssignments: [], myOpenPRs: [],
      myAssignedJiraIssues: [
        { key: "JH-1", summary: "Fix login", status: "Open", issueType: "Task", priority: "High", created: "", seen: true, fetchedAt: "", contextId: "c1", contextKey: "JH-1-ctx" },
      ],
    });
    render(<MyBoardPage />);
    await waitFor(() => expect(screen.getByRole("button", { name: /open context JH-1-ctx/i })).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: /create context from JH-1/i })).not.toBeInTheDocument();
  });

  // Org/scope is backfilled in the background (a Jira search can't return the
  // custom field), so the board must pick it up on the event instead of showing
  // an empty org chip/filter until the next manual refresh.
  it("re-reads the assignments when the org backfill reports new values", async () => {
    const { EventsOn } = await import("../../wailsjs/runtime/runtime");
    H.ListMyAssignedJiraIssues.mockResolvedValue([
      { key: "JH-1", summary: "Fix login", status: "Open", issueType: "Task", priority: "High", created: "2026-07-20T10:00:00Z", org: "rdbms", seen: false, fetchedAt: "" },
    ]);
    render(<MyBoardPage />);
    await screen.findByText("Fix login");
    expect(screen.queryByText("rdbms")).not.toBeInTheDocument();

    const handler = (EventsOn as unknown as ReturnType<typeof vi.fn>).mock.calls
      .find(([name]) => name === "board:orgs-updated")![1] as () => void;
    act(() => handler());

    // Chip on the row, and the org now offered in the filter dropdown.
    expect(await screen.findByTitle("Organization (Jira scope)")).toHaveTextContent("rdbms");
    expect(screen.getByRole("option", { name: "rdbms" })).toBeInTheDocument();
  });
});
