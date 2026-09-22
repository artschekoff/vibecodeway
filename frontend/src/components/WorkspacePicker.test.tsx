import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { WorkspacePicker } from "./WorkspacePicker";

const worktrees = [
  { repoId: "a", repoName: "acme/api", branch: "feat/x", worktreePath: "/wt/api" },
  { repoId: "b", repoName: "acme/rdbms", branch: "feat/y", worktreePath: "/wt/rdbms" },
];

describe("WorkspacePicker", () => {
  it("lists worktrees and picks the selected one", () => {
    const onPick = vi.fn();
    render(<WorkspacePicker worktrees={worktrees as never} onPick={onPick} onCancel={vi.fn()} />);
    expect(screen.getByText("acme/api")).toBeInTheDocument();
    expect(screen.getByText("acme/rdbms")).toBeInTheDocument();

    // Select the second worktree, then confirm.
    fireEvent.click(screen.getByText("acme/rdbms"));
    fireEvent.click(screen.getByText("Open"));
    expect(onPick).toHaveBeenCalledWith("/wt/rdbms");
  });

  it("cancels", () => {
    const onCancel = vi.fn();
    render(<WorkspacePicker worktrees={worktrees as never} onPick={vi.fn()} onCancel={onCancel} />);
    fireEvent.click(screen.getByText("Cancel"));
    expect(onCancel).toHaveBeenCalled();
  });

  it("ArrowDown moves the selection and Enter opens it", () => {
    const onPick = vi.fn();
    render(<WorkspacePicker worktrees={worktrees as never} onPick={onPick} onCancel={vi.fn()} />);

    fireEvent.keyDown(window, { key: "ArrowDown" });
    fireEvent.keyDown(window, { key: "Enter" });
    expect(onPick).toHaveBeenCalledWith("/wt/rdbms");
  });

  it("ArrowUp stops at the first row; Escape cancels", () => {
    const onPick = vi.fn();
    const onCancel = vi.fn();
    render(<WorkspacePicker worktrees={worktrees as never} onPick={onPick} onCancel={onCancel} />);

    fireEvent.keyDown(window, { key: "ArrowUp" });
    fireEvent.keyDown(window, { key: "Enter" });
    expect(onPick).toHaveBeenCalledWith("/wt/api");

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onCancel).toHaveBeenCalled();
  });
});
