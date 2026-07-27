import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ContextForm from "../ContextForm";
import type { Context } from "../../types";

const { matchedBranches } = vi.hoisted(() => ({
  matchedBranches: [
    { platform: "github", ownerRepo: "acme/api", branch: "feat/JH-4532", label: "" },
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
  InterpretContextUpdate: vi.fn().mockResolvedValue([]),
  ApplyContextUpdateActions: vi.fn().mockResolvedValue([]),
  AIMatchBranches: vi.fn().mockResolvedValue(matchedBranches),
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

describe("ContextForm — AI Match (Git/PRs tab)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls AIMatchBranches and pre-fills matched branches into the pending list", async () => {
    const { AIMatchBranches } = await import("../../../wailsjs/go/main/App");

    const user = userEvent.setup();
    render(<ContextForm initial={initialContext} onSave={vi.fn()} onCancel={vi.fn()} />);

    const vcsTab = screen.getByRole("button", { name: /git.*prs/i });
    await user.click(vcsTab);

    const aiMatchBtn = screen.getByRole("button", { name: /ai match/i });
    await user.click(aiMatchBtn);

    await waitFor(() => {
      expect(AIMatchBranches).toHaveBeenCalledWith("ctx-1");
    });

    expect(await screen.findByText("acme/api")).toBeInTheDocument();
    expect(screen.getByText(":feat/JH-4532")).toBeInTheDocument();
  });

  it("opens on the Git / PRs tab when initialActiveTab is 'vcs'", () => {
    render(
      <ContextForm
        initial={{ id: "ctx-1", key: "JH-1", status: "active", orgId: "o1", createdAt: "", updatedAt: "" } as any}
        initialActiveTab="vcs"
        onSave={vi.fn()}
        onCancel={vi.fn()}
      />
    );
    const vcsTab = screen.getByRole("button", { name: /git \/ prs/i });
    expect(vcsTab.className).toContain("ctx-tab--active");
  });
});
