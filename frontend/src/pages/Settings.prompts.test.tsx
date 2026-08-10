import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const H = vi.hoisted(() => ({
  ListTerminalCommands: vi.fn(),
  AddTerminalCommand: vi.fn(),
  UpdateTerminalCommand: vi.fn(),
  DeleteTerminalCommand: vi.fn(),
  GetAppSettings: vi.fn(),
  SetAppSettings: vi.fn(),
  GetKeybindings: vi.fn(),
  GetAISettings: vi.fn(),
  SetAISettings: vi.fn(),
  GetSupportedPlatforms: vi.fn(),
  GetVCSAuth: vi.fn(),
  VCSLoginStart: vi.fn(),
  VCSLoginComplete: vi.fn(),
  SetAutoLinkRepos: vi.fn(),
  UpdateKeybinding: vi.fn(),
  ListGlobalEventHooks: vi.fn(),
  ListWorkflowStatuses: vi.fn(),
  ListAIPrompts: vi.fn(),
  UpdateAIPrompt: vi.fn(),
  ResetAIPrompt: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  ListTerminalCommands: H.ListTerminalCommands,
  AddTerminalCommand: H.AddTerminalCommand,
  UpdateTerminalCommand: H.UpdateTerminalCommand,
  DeleteTerminalCommand: H.DeleteTerminalCommand,
  GetAppSettings: H.GetAppSettings,
  SetAppSettings: H.SetAppSettings,
  GetKeybindings: H.GetKeybindings,
  GetAISettings: H.GetAISettings,
  SetAISettings: H.SetAISettings,
  GetSupportedPlatforms: H.GetSupportedPlatforms,
  GetVCSAuth: H.GetVCSAuth,
  VCSLoginStart: H.VCSLoginStart,
  VCSLoginComplete: H.VCSLoginComplete,
  SetAutoLinkRepos: H.SetAutoLinkRepos,
  UpdateKeybinding: H.UpdateKeybinding,
  ListGlobalEventHooks: H.ListGlobalEventHooks,
  ListWorkflowStatuses: H.ListWorkflowStatuses,
  ListAIPrompts: H.ListAIPrompts,
  UpdateAIPrompt: H.UpdateAIPrompt,
  ResetAIPrompt: H.ResetAIPrompt,
}));

import Settings from "./Settings";
import { TabProvider } from "../contexts/TabContext";

function renderSettings() {
  return render(
    <TabProvider>
      <Settings />
    </TabProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  H.GetAppSettings.mockResolvedValue({});
  H.GetKeybindings.mockResolvedValue([]);
  H.ListTerminalCommands.mockResolvedValue([
    { id: "c1", title: "Inject", command: "/load-context <context-name>", createdAt: "", updatedAt: "" },
  ]);
  H.AddTerminalCommand.mockResolvedValue({ id: "c2", title: "", command: "", createdAt: "", updatedAt: "" });
  H.UpdateTerminalCommand.mockResolvedValue(undefined);
  H.DeleteTerminalCommand.mockResolvedValue(undefined);
  H.GetAISettings.mockResolvedValue({ provider: "openai", model: "gpt-4.1", baseUrl: "" });
  H.SetAISettings.mockResolvedValue(undefined);
  H.GetSupportedPlatforms.mockResolvedValue([]);
  H.GetVCSAuth.mockResolvedValue(null);
  H.VCSLoginStart.mockResolvedValue({});
  H.VCSLoginComplete.mockResolvedValue("");
  H.SetAutoLinkRepos.mockResolvedValue(undefined);
  H.UpdateKeybinding.mockResolvedValue(undefined);
  H.ListGlobalEventHooks.mockResolvedValue([]);
  H.ListWorkflowStatuses.mockResolvedValue([]);
  H.ListAIPrompts.mockResolvedValue([]);
  H.UpdateAIPrompt.mockResolvedValue(undefined);
  H.ResetAIPrompt.mockResolvedValue(undefined);
});

describe("Settings prompts", () => {
  it("lists prompts, edits a body, and saves", async () => {
    H.ListAIPrompts.mockResolvedValue([
      { id: "todo_action_rerequest_pr", label: "TODO Action — Rerequest PR review", body: "old body", defaultBody: "old body", placeholders: [], updatedAt: "" },
    ]);
    H.UpdateAIPrompt.mockResolvedValue(undefined);
    renderSettings();

    const ta = await screen.findByDisplayValue("old body");
    fireEvent.change(ta, { target: { value: "new body" } });
    fireEvent.click(screen.getByRole("button", { name: /Save prompts/i }));

    await waitFor(() => expect(H.UpdateAIPrompt).toHaveBeenCalledWith("todo_action_rerequest_pr", "new body"));
  });
});
