import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ContextForm from "../ContextForm";

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
}));

vi.mock("../../contexts/PassphraseContext", () => ({
  usePassphrase: () => ({
    isSet: true,
    ensurePassphrase: vi.fn().mockResolvedValue(true),
  }),
}));

describe("ContextForm — Files tab (creation mode)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("upload button is enabled when creating a new context", async () => {
    const user = userEvent.setup();
    render(<ContextForm onSave={vi.fn()} onCancel={vi.fn()} />);

    const filesTab = screen.getByRole("button", { name: /files/i });
    await user.click(filesTab);

    const uploadBtn = screen.getByRole("button", { name: /upload files/i });
    expect(uploadBtn).not.toBeDisabled();
  });

  it("does not show 'create the context first' hint during creation", async () => {
    const user = userEvent.setup();
    render(<ContextForm onSave={vi.fn()} onCancel={vi.fn()} />);

    const filesTab = screen.getByRole("button", { name: /files/i });
    await user.click(filesTab);

    expect(screen.queryByText(/create the context first/i)).not.toBeInTheDocument();
  });

  it("calls PickAndExtractFilesPreview (not PickAndExtractFiles) during creation", async () => {
    const { PickAndExtractFilesPreview, PickAndExtractFiles } = await import(
      "../../../wailsjs/go/main/App"
    );

    const user = userEvent.setup();
    render(<ContextForm onSave={vi.fn()} onCancel={vi.fn()} />);

    const filesTab = screen.getByRole("button", { name: /files/i });
    await user.click(filesTab);

    const uploadBtn = screen.getByRole("button", { name: /upload files/i });
    await user.click(uploadBtn);

    expect(PickAndExtractFilesPreview).toHaveBeenCalled();
    expect(PickAndExtractFiles).not.toHaveBeenCalled();
  });

  it("passes pending upload temp IDs to onSave", async () => {
    const { PickAndExtractFilesPreview } = await import(
      "../../../wailsjs/go/main/App"
    );
    vi.mocked(PickAndExtractFilesPreview).mockResolvedValue({
      previews: [
        {
          tempId: "tmp-1",
          originalName: "report.pdf",
          fileType: "pdf",
          extractedText: "some text",
          fileSize: 1024,
        },
      ],
      errors: [],
    } as any);

    const onSave = vi.fn().mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<ContextForm onSave={onSave} onCancel={vi.fn()} />);

    // Fill required key field
    const keyInput = screen.getByPlaceholderText(/sprint-42/i);
    await user.type(keyInput, "test-ctx");

    // Upload a file
    const filesTab = screen.getByRole("button", { name: /files/i });
    await user.click(filesTab);
    const uploadBtn = screen.getByRole("button", { name: /upload files/i });
    await user.click(uploadBtn);

    // Submit the form
    const createBtn = screen.getByRole("button", { name: /create/i });
    await user.click(createBtn);

    expect(onSave).toHaveBeenCalled();
    const callArgs = onSave.mock.calls[0];
    // pendingUploadTempIds is the 9th argument (index 8) in onSave's signature
    const pendingUploadTempIds = callArgs[8];
    expect(pendingUploadTempIds).toEqual(["tmp-1"]);
  });
});
