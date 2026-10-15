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
  SetStatsAuthorName: vi.fn(),
  UpdateKeybinding: vi.fn(),
  ListGlobalEventHooks: vi.fn(),
  ListWorkflowStatuses: vi.fn(),
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
  SetStatsAuthorName: H.SetStatsAuthorName,
  UpdateKeybinding: H.UpdateKeybinding,
  ListGlobalEventHooks: H.ListGlobalEventHooks,
  ListWorkflowStatuses: H.ListWorkflowStatuses,
}));

import Settings from "./Settings";
import { TabProvider } from "../contexts/TabContext";

async function renderSettings() {
  const r = render(
    <TabProvider>
      <Settings />
    </TabProvider>
  );
  // Settings is tabbed; terminal commands live under the Terminal tab.
  fireEvent.click(await screen.findByRole("button", { name: "Terminal" }));
  return r;
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
  H.SetStatsAuthorName.mockResolvedValue(undefined);
  H.UpdateKeybinding.mockResolvedValue(undefined);
  H.ListGlobalEventHooks.mockResolvedValue([]);
  H.ListWorkflowStatuses.mockResolvedValue([]);
});

describe("Settings terminal commands", () => {
  it("lists existing commands", async () => {
    await renderSettings();
    await waitFor(() => expect(screen.getByDisplayValue("Inject")).toBeInTheDocument());
    expect(screen.getByDisplayValue("/load-context <context-name>")).toBeInTheDocument();
  });

  it("adds a command", async () => {
    await renderSettings();
    await waitFor(() => expect(H.ListTerminalCommands).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: /add command/i }));
    await waitFor(() => expect(H.AddTerminalCommand).toHaveBeenCalledWith("", "", false));
  });

  it("saves every command with one global Save all button", async () => {
    H.ListTerminalCommands.mockResolvedValue([
      { id: "c1", title: "Inject", command: "/load", sendEnter: false, createdAt: "", updatedAt: "" },
      { id: "c2", title: "Run", command: "npm test", sendEnter: true, createdAt: "", updatedAt: "" },
    ]);
    await renderSettings();
    await waitFor(() => expect(screen.getByDisplayValue("Inject")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /save all/i }));
    await waitFor(() => {
      expect(H.UpdateTerminalCommand).toHaveBeenCalledWith("c1", "Inject", "/load", false);
      expect(H.UpdateTerminalCommand).toHaveBeenCalledWith("c2", "Run", "npm test", true);
    });
  });

  it("deletes a command", async () => {
    await renderSettings();
    await waitFor(() => expect(screen.getByDisplayValue("Inject")).toBeInTheDocument());
    fireEvent.click(screen.getAllByRole("button", { name: /delete command/i })[0]);
    await waitFor(() => expect(H.DeleteTerminalCommand).toHaveBeenCalledWith("c1"));
  });
});
