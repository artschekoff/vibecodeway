import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetVCSPRsForContext, BrowserOpenURL } = vi.hoisted(() => ({
  GetVCSPRsForContext: vi.fn(),
  BrowserOpenURL: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetVCSPRsForContext }));
vi.mock("../../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL }));

import { EditorPRInfo } from "./EditorPRInfo";

const pr = {
  id: "pr1", repoId: "r", branchId: "b", prNumber: 349, title: "Add note composer",
  author: "artschekoff", state: "open", isDraft: false, reviewStatus: "approved",
  mergeable: "MERGEABLE", ciStatus: "SUCCESS", url: "https://github.com/juna-health/juna-api/pull/349",
  headBranch: "feat_JH-2575", prCreatedAt: "", prUpdatedAt: "", fetchedAt: "",
  comments: [{ id: "c1", viewedAt: undefined }, { id: "c2", viewedAt: "2026-01-01" }],
};

beforeEach(() => { vi.clearAllMocks(); });

describe("EditorPRInfo", () => {
  it("renders PR title, number, author, head branch, unread count and badges", async () => {
    GetVCSPRsForContext.mockResolvedValue([pr]);
    render(<EditorPRInfo contextId="ctx1" />);
    await waitFor(() => expect(screen.getByText("Add note composer")).toBeInTheDocument());
    expect(screen.getByText("#349")).toBeInTheDocument();
    expect(screen.getByText("by artschekoff")).toBeInTheDocument();
    expect(screen.getByText("feat_JH-2575")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();        // 1 unread comment
    expect(screen.getByText("Approved")).toBeInTheDocument(); // via PRStatusBadge
  });

  it("opens the PR url on click", async () => {
    GetVCSPRsForContext.mockResolvedValue([pr]);
    render(<EditorPRInfo contextId="ctx1" />);
    await waitFor(() => expect(screen.getByText("Add note composer")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Add note composer"));
    expect(BrowserOpenURL).toHaveBeenCalledWith("https://github.com/juna-health/juna-api/pull/349");
  });

  it("shows only the selected repo's PR when activeRepoId is set", async () => {
    const other = { ...pr, id: "pr2", repoId: "r2", title: "Other repo PR", url: "https://x/2" };
    GetVCSPRsForContext.mockResolvedValue([pr, other]);
    render(<EditorPRInfo contextId="ctx1" activeRepoId="r2" />);
    await waitFor(() => expect(screen.getByText("Other repo PR")).toBeInTheDocument());
    expect(screen.queryByText("Add note composer")).not.toBeInTheDocument();
  });

  it("renders nothing when the selected repo has no PR", async () => {
    GetVCSPRsForContext.mockResolvedValue([pr]);
    const { container } = render(<EditorPRInfo contextId="ctx1" activeRepoId="no-such-repo" />);
    await waitFor(() => expect(GetVCSPRsForContext).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when there are no linked PRs", async () => {
    GetVCSPRsForContext.mockResolvedValue([]);
    const { container } = render(<EditorPRInfo contextId="ctx1" />);
    await waitFor(() => expect(GetVCSPRsForContext).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });
});
