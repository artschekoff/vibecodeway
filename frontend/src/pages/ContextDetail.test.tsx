import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const contextDetail = {
  context: {
    id: "ctx-1",
    key: "CTX-1",
    status: "active",
    orgId: "org-1",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    snapshotStatus: "not_started",
    snapshotIsStale: false,
  },
  issues: [],
  slackChannels: [],
  slackThreads: [],
  slackDMConfigs: [],
  contextTexts: [],
  uploadedFiles: [],
  vcsBranches: [
    {
      id: "branch-1",
      contextId: "ctx-1",
      repoId: "repo-1",
      branch: "main",
      label: "",
      createdAt: "2026-01-01T00:00:00Z",
      repo: { id: "repo-1", owner: "acme", name: "widgets", platform: "github" },
    },
  ],
  pullRequests: [
    {
      id: "pr-1",
      repoId: "repo-1",
      branchId: "branch-1",
      prNumber: 1,
      title: "Fix login bug",
      author: "alice",
      state: "open",
      isDraft: false,
      reviewStatus: "pending",
      url: "https://github.com/acme/widgets/pull/1",
      headBranch: "fix-login",
      mergeable: "true",
      ciStatus: "success",
      prCreatedAt: "2026-01-01T00:00:00Z",
      prUpdatedAt: "2026-01-01T00:00:00Z",
      fetchedAt: "2026-01-01T00:00:00Z",
      comments: [],
    },
    {
      id: "pr-2",
      repoId: "repo-1",
      branchId: "branch-1",
      prNumber: 2,
      title: "Add signup flow",
      author: "bob",
      state: "open",
      isDraft: false,
      reviewStatus: "pending",
      url: "https://github.com/acme/widgets/pull/2",
      headBranch: "add-signup",
      mergeable: "true",
      ciStatus: "success",
      prCreatedAt: "2026-01-01T00:00:00Z",
      prUpdatedAt: "2026-01-01T00:00:00Z",
      fetchedAt: "2026-01-01T00:00:00Z",
      comments: [],
    },
  ],
  relations: [],
};

// Names the App bridge exposes that ContextDetail (and the child components it
// mounts unconditionally) call during a normal render. Most just need to
// resolve to an empty/no-op value; GetContextDetail and SearchContextItems are
// the two this test drives explicitly.
const {
  GetContextDetail,
  SearchContextItems,
  ListKnownJiraStatuses,
} = vi.hoisted(() => ({
  GetContextDetail: vi.fn(),
  SearchContextItems: vi.fn(),
  ListKnownJiraStatuses: vi.fn(),
}));

vi.mock("../../wailsjs/go/main/App", () => {
  const noop = () => vi.fn().mockResolvedValue(undefined);
  const emptyArray = () => vi.fn().mockResolvedValue([]);
  return {
    GetContextDetail,
    SearchContextItems,
    AddContextRelation: noop(),
    AddIssuesToContext: noop(),
    AddSlackChannels: noop(),
    AddUserEvent: noop(),
    AddVCSBranch: noop(),
    AnalyzeContext: noop(),
    CancelRefresh: noop(),
    CheckoutInWorkspace: noop(),
    CommitStepOutput: noop(),
    CreateContextEventHook: noop(),
    DeleteContextEventHook: noop(),
    DeleteContextText: noop(),
    DeleteJiraComment: noop(),
    DeleteJiraIssue: noop(),
    DeletePersonAssignment: noop(),
    DeleteSlackDMConfig: noop(),
    DeleteSlackThread: noop(),
    DeleteUploadedFile: noop(),
    DeleteUserEvent: noop(),
    DeleteWorkspace: noop(),
    FindWorkspaceForBranch: vi.fn().mockResolvedValue(null),
    GenerateEventsFromMessages: noop(),
    GenerateHandoffPacket: noop(),
    GeneratePersonAssignments: noop(),
    GenerateSinglePersonAssignment: noop(),
    GetContextParticipants: emptyArray(),
    GetContextSnapshot: vi.fn().mockResolvedValue(null),
    GetEstimationHistory: emptyArray(),
    GetJiraInstanceURL: vi.fn().mockResolvedValue(""),
    GetPendingMessages: emptyArray(),
    GetPersonAssignmentSteps: emptyArray(),
    GetPersonAssignments: emptyArray(),
    GetUnresolvedAuthors: emptyArray(),
    GetUnviewedBreakdown: vi.fn().mockResolvedValue(null),
    ListAllWorkspaces: emptyArray(),
    ListContextEventHooks: emptyArray(),
    ListContextsByOrg: emptyArray(),
    ListEvents: emptyArray(),
    ListRepoLocalPaths: emptyArray(),
    ListSlackDMConfigs: emptyArray(),
    ListKnownJiraStatuses: ListKnownJiraStatuses.mockResolvedValue([]),
    ListWorkflowStatuses: emptyArray(),
    MarkAssignmentsOutdated: noop(),
    MarkBranchAllViewed: noop(),
    MarkCommentViewed: noop(),
    MarkContextAllViewed: noop(),
    MarkIssueAllViewed: noop(),
    MarkIssueViewed: noop(),
    MarkPRCommentViewed: noop(),
    MarkPRViewed: noop(),
    MarkSlackChannelAllViewed: noop(),
    MarkSlackDMConfigViewed: noop(),
    MarkSlackThreadViewed: noop(),
    MarkWorkspaceShouldDelete: noop(),
    OpenWorkspaceTerminal: noop(),
    RebuildAssignmentChainFrom: noop(),
    RefinePersonAssignment: noop(),
    RefreshContext: noop(),
    RegeneratePersonAssignment: noop(),
    RemoveContextRelation: noop(),
    RemoveSlackChannel: noop(),
    RemoveSlackDMEntry: noop(),
    RemoveVCSBranch: noop(),
    RunSingleAssignmentStep: noop(),
    SavePersonAssignment: noop(),
    SearchMessages: emptyArray(),
    SetContextWorkflowStatus: noop(),
    SetCurrentAssignmentStep: noop(),
    SetJiraConfig: noop(),
    ToggleCommentActive: noop(),
    ToggleContextStatus: noop(),
    ToggleContextTextActive: noop(),
    ToggleIssueActive: noop(),
    ToggleSlackChannelActive: noop(),
    ToggleSlackDMConfigActive: noop(),
    ToggleSlackThreadActive: noop(),
    ToggleUploadedFileActive: noop(),
    UpdateContext: noop(),
    UpdateUserEvent: noop(),
    UpsertContextText: noop(),
    UpsertSlackDMConfig: noop(),
    HasSlackPassphrase: vi.fn().mockResolvedValue(true),
    ValidateSlackPassphrase: noop(),
  };
});

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: () => () => {},
  EventsOff: () => {},
  BrowserOpenURL: () => {},
}));

import { TabProvider } from "../contexts/TabContext";
import { PassphraseProvider } from "../contexts/PassphraseContext";
import ContextDetailPage from "./ContextDetail";

function renderDetail() {
  return render(
    <TabProvider>
      <PassphraseProvider>
        <ContextDetailPage contextId="ctx-1" tabId="tab-1" />
      </PassphraseProvider>
    </TabProvider>
  );
}

beforeEach(() => {
  GetContextDetail.mockReset();
  SearchContextItems.mockReset();
  GetContextDetail.mockResolvedValue(contextDetail);
});

describe("ContextDetail issue status options", () => {
  const issue = {
    id: "i-1",
    contextId: "ctx-1",
    jiraKey: "JH-5353",
    summary: "Error loading metrics",
    status: "To Do",
    assignee: "",
    issueType: "Bug",
    description: "",
    jiraUpdated: "",
    parentKey: "",
    createdAt: "2026-01-01T00:00:00Z",
    isActive: true,
  };

  it("offers the statuses this Jira actually uses, not invented ones", async () => {
    // Statuses that exist in the workflow, as synced from Jira.
    ListKnownJiraStatuses.mockResolvedValue([
      "Done", "In Code Review", "In Progress", "Ready for QA", "To Do",
    ]);
    GetContextDetail.mockResolvedValue({ ...contextDetail, issues: [issue] });

    renderDetail();

    const select = await screen.findByLabelText("status of JH-5353");
    const options = Array.from(select.querySelectorAll("option")).map((o) => o.textContent);
    expect(options).toEqual(["To Do", "Done", "In Code Review", "In Progress", "Ready for QA"]);
    // "In Review" is not a status in this Jira — offering it sends a transition Jira rejects.
    expect(options).not.toContain("In Review");
  });

  it("falls back to the issue's own status when nothing has been synced yet", async () => {
    ListKnownJiraStatuses.mockResolvedValue([]);
    GetContextDetail.mockResolvedValue({ ...contextDetail, issues: [issue] });

    renderDetail();

    const select = await screen.findByLabelText("status of JH-5353");
    const options = Array.from(select.querySelectorAll("option")).map((o) => o.textContent);
    expect(options).toEqual(["To Do"]);
  });
});

describe("ContextDetail PR filtering", () => {
  it("shows all PRs when no search is active", async () => {
    renderDetail();
    expect(await screen.findByText("Fix login bug")).toBeInTheDocument();
    expect(screen.getByText("Add signup flow")).toBeInTheDocument();
  });

  it("filters the PR list to itemMatch.prIds when a search is active", async () => {
    SearchContextItems.mockResolvedValue({
      issueIds: [],
      channelIds: [],
      threadIds: [],
      textIds: [],
      fileIds: [],
      branchIds: [],
      prIds: ["pr-1"],
    });
    const user = userEvent.setup();
    renderDetail();

    await screen.findByText("Fix login bug");

    const input = screen.getByPlaceholderText("Filter items…");
    await user.type(input, "login");

    await vi.waitFor(() => expect(SearchContextItems).toHaveBeenCalled());

    expect(await screen.findByText("Fix login bug")).toBeInTheDocument();
    await vi.waitFor(() => expect(screen.queryByText("Add signup flow")).not.toBeInTheDocument());
  });
});

describe("ContextDetail topbar", () => {
  // The topbar is a fixed-height single row. Without a hideable wrapper around
  // each label the buttons shrink and wrap their text onto two lines instead of
  // collapsing, which pushes the row out of shape and squeezes the context key
  // down to a single character.
  it("keeps every action label in a .btn-label span and stays identifiable once collapsed", async () => {
    renderDetail();
    await screen.findByText("Fix login bug");

    const actions = document.querySelector(".topbar-actions");
    expect(actions).not.toBeNull();
    const buttons = Array.from(actions!.querySelectorAll("button"));
    expect(buttons.length).toBeGreaterThan(0);

    for (const button of buttons) {
      // Collapsed, the icon is all that survives — without a title the button
      // is unidentifiable.
      expect(button.getAttribute("title"), `button "${button.textContent}" needs a title`).toBeTruthy();

      // A bare text node cannot be hidden by the container query, so any label
      // has to be wrapped.
      const bareText = Array.from(button.childNodes)
        .filter((n) => n.nodeType === Node.TEXT_NODE && n.textContent?.trim())
        .map((n) => n.textContent?.trim());
      expect(bareText, `button "${button.textContent}" has an unwrapped label`).toEqual([]);
    }
  });
});
