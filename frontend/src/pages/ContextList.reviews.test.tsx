import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const {
  ListContextsByOrg,
  AuthStatus,
  GetPendingCodeReviews,
  ListTrackedRepos,
  GetSupportedPlatforms,
  ListOrganizations,
  ListWorkflowStatuses,
  MarkReviewRequestViewed,
  MarkAllReviewRequestsViewed,
} = vi.hoisted(() => ({
  ListContextsByOrg: vi.fn(),
  AuthStatus: vi.fn(),
  GetPendingCodeReviews: vi.fn(),
  ListTrackedRepos: vi.fn(),
  GetSupportedPlatforms: vi.fn(),
  ListOrganizations: vi.fn(),
  ListWorkflowStatuses: vi.fn(),
  MarkReviewRequestViewed: vi.fn(),
  MarkAllReviewRequestsViewed: vi.fn(),
}));

vi.mock("../../wailsjs/go/main/App", () => ({
  ListContextsByOrg: ListContextsByOrg.mockResolvedValue([]),
  AuthStatus: AuthStatus.mockResolvedValue("ok"),
  GetPendingCodeReviews,
  ListTrackedRepos: ListTrackedRepos.mockResolvedValue([]),
  GetSupportedPlatforms: GetSupportedPlatforms.mockResolvedValue([]),
  ListOrganizations: ListOrganizations.mockResolvedValue([]),
  ListWorkflowStatuses: ListWorkflowStatuses.mockResolvedValue([]),
  MarkReviewRequestViewed,
  MarkAllReviewRequestsViewed,
  HasSlackPassphrase: vi.fn().mockResolvedValue(true),
  ValidateSlackPassphrase: vi.fn(),
  SetContextOrg: vi.fn(),
  SetContextsOrder: vi.fn(),
  ListRepoLocalPaths: vi.fn().mockResolvedValue([]),
}));

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: () => () => {},
  EventsOff: () => {},
  BrowserOpenURL: () => {},
}));

import { TabProvider } from "../contexts/TabContext";
import { PassphraseProvider } from "../contexts/PassphraseContext";
import ContextList from "./ContextList";

// Node 25 exposes its own native `localStorage` global that lacks `.clear()`,
// shadowing jsdom's implementation. Replace it with a proper in-memory mock
// (same pattern as ContextList.test.tsx / TabContext.test.tsx).
function makeLocalStorageMock(): Storage {
  let store: Record<string, string> = {};
  return {
    get length() { return Object.keys(store).length; },
    key(index: number) { return Object.keys(store)[index] ?? null; },
    getItem(k: string) { return Object.prototype.hasOwnProperty.call(store, k) ? store[k] : null; },
    setItem(k: string, v: string) { store[k] = String(v); },
    removeItem(k: string) { delete store[k]; },
    clear() { store = {}; },
  } as Storage;
}

const localStorageMock = makeLocalStorageMock();

function renderList() {
  return render(
    <TabProvider>
      <PassphraseProvider>
        <ContextList orgId="o1" tabId="t1" />
      </PassphraseProvider>
    </TabProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("localStorage", localStorageMock);
  localStorageMock.clear();
  ListContextsByOrg.mockResolvedValue([]);
  AuthStatus.mockResolvedValue("ok");
  ListTrackedRepos.mockResolvedValue([
    { id: "repo1", platform: "github", owner: "acme", name: "widgets" },
  ]);
  GetSupportedPlatforms.mockResolvedValue([]);
  ListOrganizations.mockResolvedValue([]);
  ListWorkflowStatuses.mockResolvedValue([]);
  MarkReviewRequestViewed.mockResolvedValue(undefined);
  MarkAllReviewRequestsViewed.mockResolvedValue(undefined);
  GetPendingCodeReviews.mockResolvedValue([
    {
      id: "r1", repoId: "repo1", prNumber: 368, prTitle: "Rename headings", prAuthor: "carol",
      prUrl: "https://x/368", reviewStatus: "changes_requested", headBranch: "feat/x",
      requestedAt: "", fetchedAt: "", reviewedAt: undefined, dismissedAt: undefined, viewedAt: undefined,
      reviewers: [
        { login: "alice", state: "approved", submittedAt: "" },
        { login: "bob", state: "changes_requested", submittedAt: "" },
      ],
    },
  ]);
});

describe("Code Reviews — viewed markers + reviewers", () => {
  it("shows a new dot, reviewer badges, and marks all viewed", async () => {
    renderList();

    // Switch to Code Reviews tab.
    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    // Repo groups start collapsed — expand the repo section to reveal review cards.
    fireEvent.click(await screen.findByText("acme/widgets"));
    await waitFor(() => expect(screen.getByText("Rename headings")).toBeInTheDocument());

    expect(screen.getByTestId("review-new")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText("bob")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /mark all reviews viewed/i }));
    await waitFor(() => expect(MarkAllReviewRequestsViewed).toHaveBeenCalled());
  });
});

describe("Code Reviews — stage sections", () => {
  it("groups PRs into awaiting / re-review / approved / reviewed sections", async () => {
    GetPendingCodeReviews.mockResolvedValue([
      { id: "a", repoId: "repo1", prNumber: 1, prTitle: "Fresh one", prAuthor: "carol",
        prUrl: "https://x/1", reviewStatus: "review_required", headBranch: "b1",
        requestedAt: "", fetchedAt: "", kind: "requested" },
      { id: "b", repoId: "repo1", prNumber: 2, prTitle: "Back to me", prAuthor: "carol",
        prUrl: "https://x/2", reviewStatus: "re_review_required", headBranch: "b2",
        requestedAt: "", fetchedAt: "", kind: "requested", myReviewState: "changes_requested" },
      { id: "c", repoId: "repo1", prNumber: 3, prTitle: "All good", prAuthor: "carol",
        prUrl: "https://x/3", reviewStatus: "approved", headBranch: "b3",
        requestedAt: "", fetchedAt: "", kind: "requested", myReviewState: "approved" },
      { id: "d", repoId: "repo1", prNumber: 4, prTitle: "Author's turn", prAuthor: "carol",
        prUrl: "https://x/4", reviewStatus: "changes_requested", headBranch: "b4",
        requestedAt: "", fetchedAt: "", kind: "requested", myReviewState: "changes_requested" },
    ]);

    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    fireEvent.click(await screen.findByText("acme/widgets"));
    await waitFor(() => expect(screen.getByText("Fresh one")).toBeInTheDocument());

    expect(screen.getByTestId("review-section-awaiting_first")).toHaveTextContent("Awaiting your first review");
    expect(screen.getByTestId("review-section-re_review")).toHaveTextContent("Re-review requested");
    expect(screen.getByTestId("review-section-approved")).toHaveTextContent("Approved");
    expect(screen.getByTestId("review-section-reviewed_waiting")).toHaveTextContent("Reviewed — waiting for author");

    // My own standing decision is visible on the cards that have one.
    expect(screen.getAllByText("You: changes requested")).toHaveLength(2);
    expect(screen.getByText("You: approved")).toBeInTheDocument();
  });

  it("counts only actionable PRs on the repo header", async () => {
    GetPendingCodeReviews.mockResolvedValue([
      { id: "a", repoId: "repo1", prNumber: 1, prTitle: "Fresh one", prAuthor: "carol",
        prUrl: "https://x/1", reviewStatus: "review_required", headBranch: "b1",
        requestedAt: "", fetchedAt: "", kind: "requested" },
      { id: "c", repoId: "repo1", prNumber: 3, prTitle: "All good", prAuthor: "carol",
        prUrl: "https://x/3", reviewStatus: "approved", headBranch: "b3",
        requestedAt: "", fetchedAt: "", kind: "requested", myReviewState: "approved" },
    ]);

    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    const header = await screen.findByText("acme/widgets");
    // One awaiting_first PR is actionable; the approved one is not.
    await waitFor(() =>
      expect(header.parentElement?.querySelector(".detail-section-count")?.textContent).toBe("1"));
  });
});
