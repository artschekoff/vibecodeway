import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../../lib/monacoSetup", () => ({ setupMonaco: vi.fn() }));

let mountCb: ((editor: unknown) => void) | undefined;
vi.mock("@monaco-editor/react", () => ({
  DiffEditor: (props: { onMount?: (e: unknown) => void }) => {
    mountCb = props.onMount;
    return <div data-testid="diff-editor" />;
  },
}));

const {
  ReadRepoFile, ReadRepoFileBase, SaveRepoFile, GetEditorSettings, SetEditorSettings,
  FileBlame, FileHistory, FileDiffAtCommit, DiscardFile,
} = vi.hoisted(() => ({
  ReadRepoFile: vi.fn(), ReadRepoFileBase: vi.fn(), SaveRepoFile: vi.fn(),
  GetEditorSettings: vi.fn(), SetEditorSettings: vi.fn(),
  FileBlame: vi.fn(), FileHistory: vi.fn(), FileDiffAtCommit: vi.fn(), DiscardFile: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  ReadRepoFile, ReadRepoFileBase, SaveRepoFile, GetEditorSettings, SetEditorSettings,
  FileBlame, FileHistory, FileDiffAtCommit, DiscardFile,
}));

import { DiffEditorTab } from "./DiffEditorTab";
import { EditorSettingsProvider } from "../../contexts/EditorSettingsContext";

beforeEach(() => {
  vi.clearAllMocks();
  ReadRepoFileBase.mockResolvedValue("old\n");
  ReadRepoFile.mockResolvedValue("new\n");
  SaveRepoFile.mockResolvedValue(undefined);
  GetEditorSettings.mockResolvedValue("");
  SetEditorSettings.mockResolvedValue(undefined);
  FileBlame.mockResolvedValue([]);
  FileHistory.mockResolvedValue([{ hash: "a".repeat(40), author: "T", date: "2026-07-28", subject: "init" }]);
  FileDiffAtCommit.mockResolvedValue({ before: "a\n", after: "b\n" });
});

describe("DiffEditorTab", () => {
  it("loads head+working and saves the modified model value", async () => {
    const onSaved = vi.fn();
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={onSaved} /></EditorSettingsProvider>);

    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());
    expect(ReadRepoFileBase).toHaveBeenCalledWith("/r/api", "src/a.ts");
    expect(ReadRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts");

    // Simulate Monaco mounting with an editor whose modified model has edits.
    mountCb?.({ getModifiedEditor: () => ({ getValue: () => "edited\n" }) });

    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(SaveRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts", "edited\n"));
    expect(onSaved).toHaveBeenCalled();
  });

  it("opens the file history modal and loads the file's commits", async () => {
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={vi.fn()} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "file history" }));

    await waitFor(() => expect(FileHistory).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    expect(screen.getByText(/History:/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("init")).toBeInTheDocument());
  });

  it("fullscreen button expands the tab to a fixed overlay and back", async () => {
    const { container } = render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={vi.fn()} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());
    const root = container.firstChild as HTMLElement;
    expect(root.getAttribute("style")).not.toContain("position: fixed");

    fireEvent.click(screen.getByRole("button", { name: "fullscreen editor" }));
    expect(root.getAttribute("style")).toContain("position: fixed");

    fireEvent.click(screen.getByRole("button", { name: "exit fullscreen" }));
    expect(root.getAttribute("style")).not.toContain("position: fixed");
  });

  it("reverts the whole file via DiscardFile", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const onSaved = vi.fn();
    DiscardFile.mockResolvedValue(undefined);
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={onSaved} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    fireEvent.click(await screen.findByRole("button", { name: /revert all changes/i }));

    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });
});
