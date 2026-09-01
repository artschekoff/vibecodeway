import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

const H = vi.hoisted(() => ({
  GetPendingCodeReviews: vi.fn(),
  RefreshPendingReviews: vi.fn(),
  ListTrackedRepos: vi.fn(),
  AddTrackedRepo: vi.fn(),
  RemoveTrackedRepo: vi.fn(),
  GetSupportedPlatforms: vi.fn(),
  DismissReviewRequest: vi.fn(),
  PickRepoDirectory: vi.fn(),
  SetTrackedRepoPath: vi.fn(),
  ListAllWorkspaces: vi.fn(),
  DeleteWorkspace: vi.fn(),
  MarkWorkspaceShouldDelete: vi.fn(),
  ListRepoLocalPaths: vi.fn(),
  CheckoutInWorkspace: vi.fn(),
  FindWorkspaceForBranch: vi.fn(),
  OpenWorkspaceTerminal: vi.fn(),
}));

vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: () => () => {},
  EventsOff: () => {},
  BrowserOpenURL: () => {},
}));

import PendingCodeReviews from "./PendingCodeReviews";

beforeEach(() => {
  vi.clearAllMocks();
  H.ListTrackedRepos.mockResolvedValue([
    { id: "repo1", platform: "github", owner: "acme", name: "widgets", localPath: "" },
  ]);
  H.GetSupportedPlatforms.mockResolvedValue([]);
  H.ListAllWorkspaces.mockResolvedValue([]);
  H.ListRepoLocalPaths.mockResolvedValue([]);
  H.GetPendingCodeReviews.mockResolvedValue([
    { id: "a", repoId: "repo1", prNumber: 1, prTitle: "Fresh one", prAuthor: "carol",
      prUrl: "https://x/1", reviewStatus: "review_required", headBranch: "b1",
      requestedAt: "", fetchedAt: "", kind: "requested" },
    { id: "b", repoId: "repo1", prNumber: 2, prTitle: "Back to me", prAuthor: "carol",
      prUrl: "https://x/2", reviewStatus: "re_review_required", headBranch: "b2",
      requestedAt: "", fetchedAt: "", kind: "requested", myReviewState: "approved" },
  ]);
});

describe("PendingCodeReviews — stage sections", () => {
  it("renders a section per stage with its badge", async () => {
    render(<MemoryRouter><PendingCodeReviews /></MemoryRouter>);

    await waitFor(() => expect(screen.getByText("Fresh one")).toBeInTheDocument());
    expect(screen.getByTestId("review-section-awaiting_first")).toHaveTextContent("Awaiting your first review");
    expect(screen.getByTestId("review-section-re_review")).toHaveTextContent("Re-review requested");
    expect(screen.getByText("Re-review requested", { selector: ".pr-status-badge" })).toBeInTheDocument();
    expect(screen.getByText("You: approved")).toBeInTheDocument();
  });
});
