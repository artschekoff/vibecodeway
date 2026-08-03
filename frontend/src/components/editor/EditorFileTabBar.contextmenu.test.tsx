import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { EditorFilesContext, tabKey, type OpenTab } from "../../contexts/EditorFilesContext";
import { EditorFileTabBar } from "./EditorFileTabBar";

const mk = (path: string, pinned = false): OpenTab => ({ repoPath: "/r", repoName: "acme/r", path, status: "modified", pinned });

function renderBar(tabs: OpenTab[]) {
  const closeEditorFile = vi.fn(), closeOtherEditorFiles = vi.fn(), pinEditorFile = vi.fn();
  const value = {
    getFiles: () => ({ tabs, activeKey: tabKey(tabs[0]) }),
    openEditorFile: vi.fn(),
    closeEditorFile,
    setActiveEditorFile: vi.fn(),
    pinEditorFile,
    closeOtherEditorFiles,
  };
  render(
    <EditorFilesContext.Provider value={value}>
      <EditorFileTabBar contextId="c1" />
    </EditorFilesContext.Provider>
  );
  return { closeEditorFile, closeOtherEditorFiles, pinEditorFile };
}

describe("EditorFileTabBar right-click menu", () => {
  it("Close closes that file", () => {
    const { closeEditorFile } = renderBar([mk("a.ts"), mk("b.ts")]);
    fireEvent.contextMenu(screen.getByText("a.ts"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Close" }));
    expect(closeEditorFile).toHaveBeenCalledWith("c1", tabKey(mk("a.ts")));
  });

  it("Close all but this keeps only the right-clicked file", () => {
    const { closeOtherEditorFiles } = renderBar([mk("a.ts"), mk("b.ts")]);
    fireEvent.contextMenu(screen.getByText("a.ts"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Close all but this" }));
    expect(closeOtherEditorFiles).toHaveBeenCalledWith("c1", tabKey(mk("a.ts")));
  });

  it("Pin pins an unpinned file", () => {
    const { pinEditorFile } = renderBar([mk("a.ts")]);
    fireEvent.contextMenu(screen.getByText("a.ts"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Pin" }));
    expect(pinEditorFile).toHaveBeenCalledWith("c1", tabKey(mk("a.ts")), true);
  });
});
