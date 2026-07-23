import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ContextForm from "../ContextForm";
import type { Context } from "../../types";

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListSlackWorkspaces: vi.fn().mockResolvedValue([]),
  PickAndExtractFiles: vi.fn().mockResolvedValue([]),
  PickAndExtractFilesPreview: vi.fn().mockResolvedValue([]),
  CommitPendingUploads: vi.fn().mockResolvedValue([]),
  DiscardPendingUpload: vi.fn().mockResolvedValue(undefined),
  PreviewSlackDMs: vi.fn().mockResolvedValue(""),
  SearchJQL: vi.fn().mockResolvedValue([]),
  SearchSlackChannels: vi.fn().mockResolvedValue([]),
  DeleteUploadedFile: vi.fn().mockResolvedValue(undefined),
  GetSupportedPlatforms: vi.fn().mockResolvedValue([]),
  ListRepoBranches: vi.fn().mockResolvedValue([]),
  ListTrackedRepos: vi.fn().mockResolvedValue([]),
  ListContexts: vi.fn().mockResolvedValue([]),
  UpdateVCSRepoMeta: vi.fn().mockResolvedValue(undefined),
  InterpretContextUpdate: vi.fn().mockResolvedValue([]),
  ApplyContextUpdateActions: vi.fn().mockResolvedValue([]),
}));

vi.mock("../../contexts/PassphraseContext", () => ({
  usePassphrase: () => ({
    isSet: true,
    ensurePassphrase: vi.fn().mockResolvedValue(true),
  }),
}));

vi.mock("../ContextEventHooks", () => ({
  default: ({ contextId }: { contextId: string }) => (
    <div data-testid="context-event-hooks">hooks for {contextId}</div>
  ),
}));

const initialContext: Context = {
  id: "c1",
  key: "sprint-42",
  status: "active",
  orgId: "org-1",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  snapshotStatus: "active",
  snapshotIsStale: false,
};

function renderEditForm() {
  return render(<ContextForm initial={initialContext} onSave={vi.fn()} onCancel={vi.fn()} />);
}

describe("ContextForm — sidebar layout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders tabs inside a left sidebar and switches panels", async () => {
    const user = userEvent.setup();
    renderEditForm();

    // The tab buttons live inside a .ctx-sidebar that is inside a .ctx-body row.
    const jiraTab = screen.getByRole("button", { name: /^jira/i });
    const sidebar = jiraTab.closest(".ctx-sidebar");
    expect(sidebar).not.toBeNull();
    expect(sidebar!.closest(".ctx-body")).not.toBeNull();

    // Content column exists as a sibling of the sidebar.
    expect(sidebar!.parentElement!.querySelector(".ctx-content")).not.toBeNull();

    // Switching still works: click Relations, its panel shows.
    await user.click(screen.getByRole("button", { name: /relations/i }));
    expect(screen.getByText(/relation type/i)).toBeInTheDocument();
  });
});
