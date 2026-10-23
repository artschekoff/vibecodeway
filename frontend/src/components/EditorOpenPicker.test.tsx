import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const H = vi.hoisted(() => ({
  EnsureContextEditorWorkspace: vi.fn(),
  CheckoutInWorkspace: vi.fn(),
  EnsureBranchWorktree: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => H);

const openEditorTab = vi.fn();
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({
    openEditorTab,
    activeTabId: "t1",
    tabs: [{ id: "t1", type: "context", contextId: "ctx1", contextKey: "ENG-1", title: "ENG-1", pinned: false }],
  }),
}));

import { HotkeyProvider } from "../hotkeys/HotkeyProvider";
import { EditorOpenPicker } from "./EditorOpenPicker";

function renderPicker() {
  return render(
    <HotkeyProvider platform="darwin" keybindings={[]}>
      <EditorOpenPicker />
    </HotkeyProvider>
  );
}

describe("EditorOpenPicker", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Cmd+E with 2 branches shows a picker; ArrowDown+Enter checks out and opens the 2nd", async () => {
    H.EnsureContextEditorWorkspace.mockResolvedValue({
      ready: true,
      worktrees: [
        { repoId: "r1", repoName: "auth", branch: "main", worktreePath: "/wt/a" },
        { repoId: "r1", repoName: "auth", branch: "feat", worktreePath: "" },
      ],
      choices: [],
    });
    H.EnsureBranchWorktree.mockResolvedValue("/wt/b");
    renderPicker();
    fireEvent.keyDown(window, { key: "e", metaKey: true });
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(H.EnsureBranchWorktree).not.toHaveBeenCalled();
    fireEvent.keyDown(window, { key: "ArrowDown" });
    fireEvent.keyDown(window, { key: "Enter" });
    await waitFor(() => expect(openEditorTab).toHaveBeenCalledWith("ctx1", "ENG-1", "/wt/b"));
    expect(H.EnsureBranchWorktree).toHaveBeenCalledWith("r1", "feat");
  });
});
