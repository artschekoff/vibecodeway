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
vi.mock("../components/editor/EditorPRInfo", () => ({
  EditorPRInfo: ({ contextId }: { contextId: string }) => <div data-testid="pr-info">{contextId}</div>,
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

  it("shows the context key and Context Editing mode in the header", async () => {
    render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="JH-3940" />
      </EditorFilesProvider>
    );
    await waitFor(() => expect(screen.getByText("JH-3940")).toBeInTheDocument());
    expect(screen.getByText("Context Editing")).toBeInTheDocument();
  });

  it("shows PR Review mode for a PR-review context key", async () => {
    render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="juna-health/juna-api#4521" />
      </EditorFilesProvider>
    );
    await waitFor(() => expect(screen.getByText("juna-health/juna-api#4521")).toBeInTheDocument());
    expect(screen.getByText("PR Review")).toBeInTheDocument();
  });

  it("renders the PR info block for the context", async () => {
    render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="juna-health/juna-api#349" />
      </EditorFilesProvider>
    );
    await waitFor(() => expect(screen.getByTestId("pr-info")).toHaveTextContent("c1"));
  });
});
