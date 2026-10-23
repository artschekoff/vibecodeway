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
  UpdatePRBranch,
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
  UpdatePRBranch: vi.fn(),
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
  UpdatePRBranch,
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
      id: "a1", repoId: "repo1", prNumber: 42, prTitle: "My feature", prAuthor: "me",
      prUrl: "https://x/42", reviewStatus: "changes_requested", headBranch: "feat/x",
      requestedAt: "", fetchedAt: "", reviewedAt: undefined, dismissedAt: undefined, viewedAt: undefined,
      mergeable: "CONFLICTING", ciStatus: "FAILURE", kind: "authored",
      reviewers: [{ login: "bob", state: "changes_requested", submittedAt: "" }],
    },
  ]);
});

describe("Code Reviews — authored PRs (My PRs)", () => {
  it("shows an authored PR under My PRs with CI + conflict + reviewer chip", async () => {
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    fireEvent.click(await screen.findByText("acme/widgets"));

    expect(await screen.findByText("My feature")).toBeInTheDocument();
    expect(screen.getByText("My PRs", { selector: ".review-waiting-header" })).toBeInTheDocument();
    expect(screen.getByText("bob")).toBeInTheDocument();
    expect(screen.getByText(/CI failed/i)).toBeInTheDocument();
    expect(screen.getByText(/Conflicts/i)).toBeInTheDocument();
  });
  it("the My PRs checkbox hides requested reviews and keeps authored PRs", async () => {
    GetPendingCodeReviews.mockResolvedValue([
      {
        id: "a1", repoId: "repo1", prNumber: 42, prTitle: "My feature", prAuthor: "me",
        prUrl: "https://x/42", reviewStatus: "changes_requested", headBranch: "feat/x",
        requestedAt: "", fetchedAt: "", reviewedAt: undefined, dismissedAt: undefined, viewedAt: undefined,
        mergeable: "CONFLICTING", ciStatus: "FAILURE", kind: "authored", reviewers: [],
      },
      {
        id: "r1", repoId: "repo1", prNumber: 43, prTitle: "Someone else's PR", prAuthor: "alice",
        prUrl: "https://x/43", reviewStatus: "pending", headBranch: "feat/y",
        requestedAt: "", fetchedAt: "", reviewedAt: undefined, dismissedAt: undefined, viewedAt: undefined,
        mergeable: "MERGEABLE", ciStatus: "SUCCESS", kind: "requested", reviewers: [],
      },
    ]);
    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    fireEvent.click(await screen.findByText("acme/widgets"));
    expect(await screen.findByText("My feature")).toBeInTheDocument();
    expect(screen.getByText("Someone else's PR")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: /my prs only/i }));
    expect(screen.getByText("My feature")).toBeInTheDocument();
    expect(screen.queryByText("Someone else's PR")).not.toBeInTheDocument();
    expect(localStorage.getItem("vibecodeway:reviews:myPRsOnly")).toBe("1");

    fireEvent.click(screen.getByRole("checkbox", { name: /my prs only/i }));
    expect(await screen.findByText("Someone else's PR")).toBeInTheDocument();
  });

  it("offers the editor-opening 'Review with AI' button on an authored PR", async () => {
    renderList();

    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    fireEvent.click(await screen.findByText("acme/widgets"));
    await screen.findByText("My feature");

    expect(screen.getByRole("button", { name: /review with ai/i })).toBeInTheDocument();
  });

  it("Update branch button calls UpdatePRBranch and settles into Updated ✓", async () => {
    GetPendingCodeReviews.mockResolvedValue([
      {
        id: "a1", repoId: "repo1", prNumber: 42, prTitle: "My feature", prAuthor: "me",
        prUrl: "https://x/42", reviewStatus: "", headBranch: "feat/x",
        requestedAt: "", fetchedAt: "", kind: "authored",
        mergeStateStatus: "BEHIND",
      },
    ]);
    let resolveUpdate: () => void = () => {};
    UpdatePRBranch.mockImplementation(() => new Promise<void>((res) => { resolveUpdate = res; }));

    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    fireEvent.click(await screen.findByText("acme/widgets"));
    await screen.findByText("My feature");

    fireEvent.click(screen.getByRole("button", { name: /update branch/i }));
    expect(UpdatePRBranch).toHaveBeenCalledWith("repo1", 42);
    expect(await screen.findByText(/updating…/i)).toBeInTheDocument();

    resolveUpdate();
    expect(await screen.findByText(/updated ✓/i)).toBeInTheDocument();
  });

  it("failed Update branch shows Update failed and stays clickable", async () => {
    GetPendingCodeReviews.mockResolvedValue([
      {
        id: "a1", repoId: "repo1", prNumber: 42, prTitle: "My feature", prAuthor: "me",
        prUrl: "https://x/42", reviewStatus: "", headBranch: "feat/x",
        requestedAt: "", fetchedAt: "", kind: "authored",
        mergeStateStatus: "BEHIND",
      },
    ]);
    UpdatePRBranch.mockRejectedValue(new Error("merge conflict"));

    renderList();
    fireEvent.click(await screen.findByRole("button", { name: /code reviews/i }));
    fireEvent.click(await screen.findByText("acme/widgets"));
    await screen.findByText("My feature");

    fireEvent.click(screen.getByRole("button", { name: /update branch/i }));
    const failed = await screen.findByRole("button", { name: /update failed/i });
    expect(failed).toBeEnabled();
    expect(failed.title).toMatch(/merge conflict/);
  });
});
