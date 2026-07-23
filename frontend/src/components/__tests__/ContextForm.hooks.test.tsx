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

describe("ContextForm — Hooks tab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the mocked ContextEventHooks scoped to the context when the Hooks tab is selected", async () => {
    const user = userEvent.setup();
    render(<ContextForm initial={initialContext} onSave={vi.fn()} onCancel={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /hooks/i }));

    expect(screen.getByTestId("context-event-hooks")).toHaveTextContent("hooks for c1");
  });

  it("does not show the Hooks tab when creating a new context", () => {
    render(<ContextForm onSave={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /hooks/i })).not.toBeInTheDocument();
  });
});
