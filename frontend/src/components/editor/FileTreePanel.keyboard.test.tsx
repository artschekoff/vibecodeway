import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetRepoFileTree } = vi.hoisted(() => ({ GetRepoFileTree: vi.fn() }));
vi.mock("../../../wailsjs/go/main/App", () => ({ GetRepoFileTree }));

import { FileTreePanel } from "./FileTreePanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";

function renderTree(onOpenFile = vi.fn()) {
  render(
    <HotkeyProvider platform="darwin">
      <FileTreePanel repoPath="/r/api" repoName="acme/api" onOpenFile={onOpenFile} />
    </HotkeyProvider>
  );
  return onOpenFile;
}

beforeEach(() => {
  vi.clearAllMocks();
  GetRepoFileTree.mockResolvedValue(["src/a.ts", "b.ts"]);
});

describe("FileTreePanel keyboard navigation", () => {
  it("ArrowDown highlights first row; ArrowRight expands+steps in; Space opens the file", async () => {
    const onOpenFile = renderTree();
    await waitFor(() => expect(screen.getByText("src/")).toBeInTheDocument());

    const tree = screen.getByText("src/").parentElement!.parentElement!; // scroll container
    tree.focus();

    fireEvent.keyDown(tree, { key: "ArrowDown" });
    expect(screen.getByText("src/").closest(".file-row--active")).not.toBeNull();

    fireEvent.keyDown(tree, { key: "ArrowRight" }); // expand src/ and step into a.ts
    await waitFor(() => expect(screen.getByText("a.ts")).toBeInTheDocument());

    fireEvent.keyDown(tree, { key: " " }); // Space opens the active file
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/a.ts", "", "head");
  });

  it("⌘F F focuses the filter input; ⌘R reloads the tree", async () => {
    renderTree();
    await waitFor(() => expect(screen.getByText("src/")).toBeInTheDocument());

    const tree = screen.getByText("src/").parentElement!.parentElement!;
    fireEvent.focus(tree); // gives the FILES zone its scope (flushes state + effect)

    // ⌘F then F -> focus file filter.
    fireEvent.keyDown(window, { key: "f", metaKey: true });
    fireEvent.keyDown(window, { key: "f" });
    expect(document.activeElement).toBe(screen.getByLabelText("filter repo files"));

    expect(GetRepoFileTree).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(window, { key: "r", metaKey: true }); // ⌘R reload
    await waitFor(() => expect(GetRepoFileTree).toHaveBeenCalledTimes(2));
  });
});
