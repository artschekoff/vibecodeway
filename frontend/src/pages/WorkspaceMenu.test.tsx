import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WorkspaceMenu } from "./ContextDetail";
import type { VCSWorkspace } from "../types";

const workspace: VCSWorkspace = {
  id: "ws-1",
  repoPathId: "rp-1",
  repoId: "repo-1",
  branch: "feat/example",
  worktreePath: "/Users/dev/code/example/.worktrees/feat-example",
  status: "active",
  createdAt: "2026-07-17T00:00:00Z",
  updatedAt: "2026-07-17T00:00:00Z",
};

describe("WorkspaceMenu — Copy local path", () => {
  it("copies worktreePath to the clipboard when Copy local path is clicked", async () => {
    const user = userEvent.setup();

    // Spy on clipboard after userEvent.setup() has initialized it
    const writeTextSpy = vi.spyOn(navigator.clipboard, "writeText");

    render(
      <WorkspaceMenu
        workspace={workspace}
        checkoutLoading={false}
        onMarkDone={vi.fn()}
        onDelete={vi.fn()}
        onCheckout={vi.fn()}
      />,
    );

    const triggerButton = screen.getByTitle("Workspace actions");
    await user.click(triggerButton);

    const copyButton = await screen.findByText("Copy local path");
    await user.click(copyButton);

    expect(writeTextSpy).toHaveBeenCalledWith(
      "/Users/dev/code/example/.worktrees/feat-example",
    );
  });
});
