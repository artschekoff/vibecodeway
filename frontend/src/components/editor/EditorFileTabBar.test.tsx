import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { EditorFilesContext, tabKey, type OpenTab } from "../../contexts/EditorFilesContext";
import { EditorFileTabBar } from "./EditorFileTabBar";

const file = (path: string): OpenTab => ({ repoPath: "/r", repoName: "acme/r", path, status: "modified" });

function renderWithFiles(tabs: OpenTab[], activeKey: string) {
  const setActiveEditorFile = vi.fn();
  const closeEditorFile = vi.fn();
  const value = {
    getFiles: () => ({ tabs, activeKey }),
    openEditorFile: vi.fn(),
    closeEditorFile,
    setActiveEditorFile,
    pinEditorFile: vi.fn(),
    closeAllEditorFiles: vi.fn(),
  };
  render(
    <EditorFilesContext.Provider value={value}>
      <EditorFileTabBar contextId="c1" />
    </EditorFilesContext.Provider>
  );
  return { setActiveEditorFile, closeEditorFile };
}

describe("EditorFileTabBar", () => {
  it("renders a tab per open file", () => {
    renderWithFiles([file("dir/a.ts"), file("b.ts")], tabKey(file("b.ts")));
    expect(screen.getByText("a.ts")).toBeInTheDocument();
    expect(screen.getByText("b.ts")).toBeInTheDocument();
  });

  it("activates a file on click and closes on ×", () => {
    const { setActiveEditorFile, closeEditorFile } = renderWithFiles(
      [file("a.ts"), file("b.ts")],
      tabKey(file("b.ts"))
    );
    fireEvent.click(screen.getByText("a.ts"));
    expect(setActiveEditorFile).toHaveBeenCalledWith("c1", tabKey(file("a.ts")));
    fireEvent.click(screen.getByLabelText("close a.ts"));
    expect(closeEditorFile).toHaveBeenCalledWith("c1", tabKey(file("a.ts")));
  });

  it("renders nothing when there are no open files", () => {
    const { container } = render(
      <EditorFilesContext.Provider
        value={{ getFiles: () => ({ tabs: [], activeKey: "" }), openEditorFile: vi.fn(), closeEditorFile: vi.fn(), setActiveEditorFile: vi.fn(), pinEditorFile: vi.fn(), closeAllEditorFiles: vi.fn() }}
      >
        <EditorFileTabBar contextId="c1" />
      </EditorFilesContext.Provider>
    );
    expect(container).toBeEmptyDOMElement();
  });
});
