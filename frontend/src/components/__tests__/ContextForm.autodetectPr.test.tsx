import { render, screen, waitFor } from "@testing-library/react";
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
  ListTrackedRepos: vi.fn().mockResolvedValue([{ id: "r1", platform: "github", owner: "acme", name: "api", localPath: "" }]),
  ListContexts: vi.fn().mockResolvedValue([]),
  UpdateVCSRepoMeta: vi.fn().mockResolvedValue(undefined),
  AIMatchBranches: vi.fn().mockResolvedValue([]),
}));

vi.mock("../../contexts/PassphraseContext", () => ({
  usePassphrase: () => ({ isSet: true, ensurePassphrase: vi.fn().mockResolvedValue(true) }),
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

/** Picks the tracked repo from the repository dropdown and clicks Add. */
async function addBranch(user: ReturnType<typeof userEvent.setup>, ownerRepo: string) {
  await user.click(screen.getByRole("button", { name: /select repository/i }));
  await user.click(await screen.findByRole("button", { name: ownerRepo }));
  await user.click(screen.getByRole("button", { name: /^add$/i }));
}

describe("ContextForm — Autodetect PR", () => {
  beforeEach(() => vi.clearAllMocks());

  it("is checked by default and links the branch with PR resolution on", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(<ContextForm initial={initialContext} onSave={onSave} onCancel={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /git.*prs/i }));
    expect(screen.getByRole("checkbox", { name: /autodetect pr/i })).toBeChecked();

    await addBranch(user, "acme/api");
    await user.click(screen.getByRole("button", { name: /^save changes$/i }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const branches = onSave.mock.calls[0][9];
    expect(branches).toEqual([expect.objectContaining({ ownerRepo: "acme/api", autodetectPr: true })]);
  });

  it("links the branch without PR resolution when unchecked", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(<ContextForm initial={initialContext} onSave={onSave} onCancel={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /git.*prs/i }));
    await user.click(screen.getByRole("checkbox", { name: /autodetect pr/i }));

    await addBranch(user, "acme/api");
    await user.click(screen.getByRole("button", { name: /^save changes$/i }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const branches = onSave.mock.calls[0][9];
    expect(branches).toEqual([expect.objectContaining({ ownerRepo: "acme/api", autodetectPr: false })]);
    // The pending row flags that this link won't pull PRs.
    expect(screen.getByText("no PR")).toBeInTheDocument();
  });

  it("resets to checked after each add so the safe default doesn't stick off", async () => {
    const user = userEvent.setup();
    render(<ContextForm initial={initialContext} onSave={vi.fn()} onCancel={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /git.*prs/i }));
    await user.click(screen.getByRole("checkbox", { name: /autodetect pr/i }));
    await addBranch(user, "acme/api");

    expect(screen.getByRole("checkbox", { name: /autodetect pr/i })).toBeChecked();
  });
});
