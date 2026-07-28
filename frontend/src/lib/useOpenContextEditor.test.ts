import { renderHook, act, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { EnsureContextEditorWorkspace, CheckoutInWorkspace } = vi.hoisted(() => ({
  EnsureContextEditorWorkspace: vi.fn(),
  CheckoutInWorkspace: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ EnsureContextEditorWorkspace, CheckoutInWorkspace }));

import { useOpenContextEditor } from "./useOpenContextEditor";

beforeEach(() => {
  vi.clearAllMocks();
  CheckoutInWorkspace.mockResolvedValue(undefined);
});

describe("useOpenContextEditor", () => {
  it("opens the editor directly when a workspace is ready", async () => {
    EnsureContextEditorWorkspace.mockResolvedValue({ ready: true, choices: [] });
    const openEditorTab = vi.fn();
    const { result } = renderHook(() => useOpenContextEditor(openEditorTab));

    await act(async () => { await result.current.open("c1", "KEY-1"); });

    expect(openEditorTab).toHaveBeenCalledWith("c1", "KEY-1");
    expect(result.current.picker).toBeNull();
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
