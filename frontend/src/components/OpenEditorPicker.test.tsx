import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { ListLocalBranches } = vi.hoisted(() => ({ ListLocalBranches: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({ ListLocalBranches }));

import { OpenEditorPicker } from "./OpenEditorPicker";

const choices = [
  { repoId: "r1", repoName: "acme/api", paths: [{ id: "p1", path: "/r/api" }] },
];

beforeEach(() => {
  vi.clearAllMocks();
  ListLocalBranches.mockResolvedValue([
    { name: "main", isCurrent: false, isGone: false },
    { name: "feat/x", isCurrent: true, isGone: false },
  ]);
});

describe("OpenEditorPicker", () => {
  it("loads local branches and confirms with the current branch by default", async () => {
    const onConfirm = vi.fn();
    render(<OpenEditorPicker choices={choices as never} onConfirm={onConfirm} onCancel={vi.fn()} />);

    await waitFor(() => expect(ListLocalBranches).toHaveBeenCalledWith("p1"));
    // Current branch is preselected in the option list.
    await waitFor(() => expect(screen.getByText("feat/x (current)")).toBeInTheDocument());

    fireEvent.click(screen.getByText("Checkout & Open"));
    expect(onConfirm).toHaveBeenCalledWith("p1", "feat/x");
  });

  it("cancels", () => {
    const onCancel = vi.fn();
    render(<OpenEditorPicker choices={choices as never} onConfirm={vi.fn()} onCancel={onCancel} />);
    fireEvent.click(screen.getByText("Cancel"));
    expect(onCancel).toHaveBeenCalled();
  });
});
