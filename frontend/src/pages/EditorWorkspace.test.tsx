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
  EditorPRInfo: ({ contextId, activeRepoId }: { contextId: string; activeRepoId?: string }) => (
    <div data-testid="pr-info">{contextId}:{activeRepoId ?? "none"}</div>
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

  it("root fills its flex container instead of shrink-wrapping to content", async () => {
    // TabContent mounts each editor tab inside a `display:flex` wrapper. Without
    // `flex:1` on our root it becomes a flex item with flex-grow:0 and shrink-
    // wraps to content width — collapsing the diff pane and letting the sidebar
    // take over the width (regression from the keep-tabs-mounted change).
    const { container } = render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toBeInTheDocument());
    const style = (container.firstChild as HTMLElement).getAttribute("style") ?? "";
    expect(style).toContain("flex: 1");
    expect(style).toContain("min-width: 0");
  });

  it("opens the terminal at initialCwd when provided", async () => {
    GetContextChanges.mockResolvedValue([
      { repoId: "id-api", repoName: "acme/api", repoPath: "/r/api", files: [] },
      { repoId: "id-rdbms", repoName: "acme/rdbms", repoPath: "/r/rdbms", files: [] },
    ]);
    render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" initialCwd="/r/rdbms" />
      </EditorFilesProvider>
    );
    // Terminal mock renders its cwd as text.
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/rdbms"));
  });

  it("passes the selected repo's id to the PR head and updates it when the selector changes", async () => {
    GetContextChanges.mockResolvedValue([
      { repoId: "id-api", repoName: "acme/api", repoPath: "/r/api", files: [] },
      { repoId: "id-rdbms", repoName: "acme/rdbms", repoPath: "/r/rdbms", files: [] },
    ]);
    render(
      <EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider>
    );
    // Defaults to the first repo's id.
    await waitFor(() => expect(screen.getByTestId("pr-info")).toHaveTextContent("c1:id-api"));

    // Switching the repo selector re-targets the PR head at the new repo.
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "/r/rdbms" } });
    await waitFor(() => expect(screen.getByTestId("pr-info")).toHaveTextContent("c1:id-rdbms"));
  });
});
