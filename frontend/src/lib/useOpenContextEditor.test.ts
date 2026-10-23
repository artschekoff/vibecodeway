import { renderHook, act, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { EnsureContextEditorWorkspace, CheckoutInWorkspace, EnsureBranchWorktree } = vi.hoisted(() => ({
  EnsureContextEditorWorkspace: vi.fn(),
  CheckoutInWorkspace: vi.fn(),
  EnsureBranchWorktree: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ EnsureContextEditorWorkspace, CheckoutInWorkspace, EnsureBranchWorktree }));

import { useOpenContextEditor } from "./useOpenContextEditor";

beforeEach(() => {
  vi.clearAllMocks();
  CheckoutInWorkspace.mockResolvedValue(undefined);
  // The checkout happens only for the picked branch, after the pick.
  EnsureBranchWorktree.mockImplementation(async (_repo: string, branch: string) => `/wt/${branch}`);
});

describe("useOpenContextEditor", () => {
  it("checks out the single linked branch and opens the editor there", async () => {
    EnsureContextEditorWorkspace.mockResolvedValue({
      ready: true,
      worktrees: [{ repoId: "a", repoName: "r", branch: "b", worktreePath: "" }],
      choices: [],
    });
    const openEditorTab = vi.fn();
    const { result } = renderHook(() => useOpenContextEditor(openEditorTab));

    await act(async () => { await result.current.open("c1", "KEY-1"); });

    expect(EnsureBranchWorktree).toHaveBeenCalledWith("a", "b");
    expect(openEditorTab).toHaveBeenCalledWith("c1", "KEY-1", "/wt/b");
    expect(result.current.picker).toBeNull();
  });

  it("shows the branch picker when ≥2 branches are linked and checks out only the pick", async () => {
    EnsureContextEditorWorkspace.mockResolvedValue({
      ready: true,
      worktrees: [
        { repoId: "a", repoName: "acme/api", branch: "feat/x", worktreePath: "/wt/api" },
        { repoId: "b", repoName: "acme/rdbms", branch: "feat/y", worktreePath: "" },
      ],
      choices: [],
    });
    const openEditorTab = vi.fn();
    const { result } = renderHook(() => useOpenContextEditor(openEditorTab));

    await act(async () => { await result.current.open("c1", "KEY-1"); });

    expect(openEditorTab).not.toHaveBeenCalled();
    expect(EnsureBranchWorktree).not.toHaveBeenCalled();
    expect(result.current.workspacePicker?.worktrees).toHaveLength(2);

    // Picking a branch checks out just that one and opens the editor there.
    await act(async () => { await result.current.confirmWorkspace(result.current.workspacePicker!.worktrees[1]); });
    expect(EnsureBranchWorktree).toHaveBeenCalledTimes(1);
    expect(EnsureBranchWorktree).toHaveBeenCalledWith("b", "feat/y");
    expect(openEditorTab).toHaveBeenCalledWith("c1", "KEY-1", "/wt/feat/y");
    expect(result.current.workspacePicker).toBeNull();
  });

  it("shows the picker when no workspace is ready but choices exist", async () => {
    EnsureContextEditorWorkspace.mockResolvedValue({
      ready: false,
      choices: [{ repoId: "r1", repoName: "acme/api", paths: [{ id: "p1", path: "/r/api" }] }],
    });
    const openEditorTab = vi.fn();
    const { result } = renderHook(() => useOpenContextEditor(openEditorTab));

    await act(async () => { await result.current.open("c1", "KEY-1"); });

    expect(openEditorTab).not.toHaveBeenCalled();
    expect(result.current.picker?.choices).toHaveLength(1);

    // Confirming a pick checks out then opens the editor.
    await act(async () => { await result.current.confirmPick("p1", "feat/x"); });
    expect(CheckoutInWorkspace).toHaveBeenCalledWith("p1", "feat/x");
    expect(openEditorTab).toHaveBeenCalledWith("c1", "KEY-1");
    await waitFor(() => expect(result.current.picker).toBeNull());
  });

  it("surfaces an error when there is no checkout-able repo", async () => {
    EnsureContextEditorWorkspace.mockResolvedValue({ ready: false, choices: [] });
    const openEditorTab = vi.fn();
    const { result } = renderHook(() => useOpenContextEditor(openEditorTab));

    await act(async () => { await result.current.open("c1", "KEY-1"); });

    expect(openEditorTab).not.toHaveBeenCalled();
    expect(result.current.picker).toBeNull();
    expect(result.current.error).toMatch(/local path/i);
  });
});
