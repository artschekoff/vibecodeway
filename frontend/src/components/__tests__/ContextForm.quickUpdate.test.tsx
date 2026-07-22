import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ContextForm from "../ContextForm";
import type { Context } from "../../types";

const { proposedActions, applyResults } = vi.hoisted(() => ({
  proposedActions: [
    { action: "set_status", statusSlug: "in_review", description: "Set status → In Review" },
    { action: "add_event", eventType: "work_completed", summary: "perf fixed", description: "Add work_completed: perf fixed" },
  ],
  applyResults: [
    { action: "set_status", ok: true, message: "" },
    { action: "add_event", ok: true, message: "" },
  ],
}));

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
  InterpretContextUpdate: vi.fn().mockResolvedValue(proposedActions),
  ApplyContextUpdateActions: vi.fn().mockResolvedValue(applyResults),
}));

vi.mock("../../contexts/PassphraseContext", () => ({
  usePassphrase: () => ({
    isSet: true,
    ensurePassphrase: vi.fn().mockResolvedValue(true),
  }),
}));

const initialContext: Context = {
  id: "ctx-1",
  key: "sprint-42",
  status: "active",
  orgId: "org-1",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  snapshotStatus: "active",
  snapshotIsStale: false,
};

describe("ContextForm — Quick Update tab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("interprets a note into proposed actions and applies the selected ones", async () => {
    const { InterpretContextUpdate, ApplyContextUpdateActions } = await import(
      "../../../wailsjs/go/main/App"
    );

    const user = userEvent.setup();
    render(<ContextForm initial={initialContext} onSave={vi.fn()} onCancel={vi.fn()} />);

    const quickTab = screen.getByRole("button", { name: /quick update/i });
    await user.click(quickTab);

    const textarea = screen.getByPlaceholderText(/move this to in review/i);
    await user.type(textarea, "Mark it in review, note perf fixed");

    const interpretBtn = screen.getByRole("button", { name: /interpret/i });
    await user.click(interpretBtn);

    await waitFor(() => {
      expect(InterpretContextUpdate).toHaveBeenCalledWith("ctx-1", "Mark it in review, note perf fixed");
    });

    const checkbox1 = await screen.findByRole("checkbox", { name: /set status → in review/i });
    const checkbox2 = screen.getByRole("checkbox", { name: /add work_completed: perf fixed/i });
    expect(checkbox1).toBeChecked();
    expect(checkbox2).toBeChecked();

    const applyBtn = screen.getByRole("button", { name: /apply selected/i });
    await user.click(applyBtn);

    await waitFor(() => {
      expect(ApplyContextUpdateActions).toHaveBeenCalledWith("ctx-1", proposedActions);
    });
  });

  it("does not show the Quick Update tab when creating a new context", () => {
    render(<ContextForm onSave={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /quick update/i })).not.toBeInTheDocument();
  });
});
