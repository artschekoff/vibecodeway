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
