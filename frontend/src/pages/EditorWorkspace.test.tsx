import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../components/editor/Terminal", () => ({ Terminal: ({ cwd }: { cwd: string }) => <div data-testid="terminal">{cwd}</div> }));
vi.mock("../components/editor/DiffEditorTab", () => ({
  DiffEditorTab: ({ path }: { path: string }) => <div data-testid="diff">{path}</div>,
}));
vi.mock("../components/editor/ChangedFilesPanel", () => ({
  ChangedFilesPanel: ({ onOpenFile }: { onOpenFile: (rp: string, rn: string, p: string, s: string) => void }) => (
    <button onClick={() => onOpenFile("/r/api", "acme/api", "src/a.ts", "modified")}>open-a</button>
  ),
}));

const { GetContextChanges } = vi.hoisted(() => ({ GetContextChanges: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({ GetContextChanges }));

import { EditorWorkspace } from "./EditorWorkspace";
import { EditorFilesProvider } from "../contexts/EditorFilesContext";

beforeEach(() => {
  vi.clearAllMocks();
  GetContextChanges.mockResolvedValue([{ repoName: "acme/api", repoPath: "/r/api", files: [] }]);
});

describe("EditorWorkspace", () => {
  it("starts terminal at first repo and opens a diff tab on file click", async () => {
    render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));

    fireEvent.click(screen.getByText("open-a"));
    await waitFor(() => expect(screen.getByTestId("diff")).toHaveTextContent("src/a.ts"));
  });
});
