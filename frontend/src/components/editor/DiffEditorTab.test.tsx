import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../../lib/monacoSetup", () => ({ setupMonaco: vi.fn() }));

let mountCb: ((editor: unknown, monaco?: unknown) => void) | undefined;
vi.mock("@monaco-editor/react", () => ({
  DiffEditor: (props: { onMount?: (e: unknown, m?: unknown) => void }) => {
    mountCb = props.onMount;
    return <div data-testid="diff-editor" />;
  },
}));

const {
  ReadRepoFile, ReadRepoFileBase, ReadRepoFileHead, SaveRepoFile, GetEditorSettings, SetEditorSettings,
  FileBlame, FileHistory, FileDiffAtCommit, DiscardFile, ConfirmAction,
} = vi.hoisted(() => ({
  ReadRepoFile: vi.fn(), ReadRepoFileBase: vi.fn(), ReadRepoFileHead: vi.fn(), SaveRepoFile: vi.fn(),
  GetEditorSettings: vi.fn(), SetEditorSettings: vi.fn(),
  FileBlame: vi.fn(), FileHistory: vi.fn(), FileDiffAtCommit: vi.fn(), DiscardFile: vi.fn(),
  ConfirmAction: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  ReadRepoFile, ReadRepoFileBase, ReadRepoFileHead, SaveRepoFile, GetEditorSettings, SetEditorSettings,
  FileBlame, FileHistory, FileDiffAtCommit, DiscardFile, ConfirmAction,
}));

import { DiffEditorTab } from "./DiffEditorTab";
import { EditorSettingsProvider } from "../../contexts/EditorSettingsContext";

beforeEach(() => {
  vi.clearAllMocks();
  ReadRepoFileBase.mockResolvedValue("old\n");
  ReadRepoFileHead.mockResolvedValue("head\n");
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
    expect(ReadRepoFileHead).not.toHaveBeenCalled();
    expect(ReadRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts");

    // Simulate Monaco mounting with an editor whose modified model has edits.
    mountCb?.({ getModifiedEditor: () => ({ getValue: () => "edited\n" }) });

    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(SaveRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts", "edited\n"));
    expect(onSaved).toHaveBeenCalled();
  });

  it("re-centers the tour anchor line after the diff layout lands (onDidUpdateDiff)", async () => {
    ReadRepoFile.mockResolvedValue("line1\nline2\nANCHOR here\nline4\n");
    const revealLineInCenter = vi.fn();
    let diffListener: (() => void) | undefined;
    const disposeSpy = vi.fn();

    render(
      <EditorSettingsProvider>
        <DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={vi.fn()} highlightAnchor="ANCHOR here" />
      </EditorSettingsProvider>
    );
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    mountCb?.(
      {
        getModifiedEditor: () => ({
          getValue: () => "",
          deltaDecorations: vi.fn().mockReturnValue([]),
          revealLineInCenter,
        }),
        onDidUpdateDiff: (l: () => void) => { diffListener = l; return { dispose: disposeSpy }; },
      },
      { Range: class { constructor() { /* range stub */ } } }
    );

    // Immediate reveal at the anchor's line (3) once content + editor are ready.
    await waitFor(() => expect(revealLineInCenter).toHaveBeenCalledWith(3));

    // Alignment view zones land later and shift line positions — the diff
    // update must re-center the same line, then unsubscribe.
    diffListener?.();
    expect(revealLineInCenter).toHaveBeenLastCalledWith(3);
    expect(revealLineInCenter.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(disposeSpy).toHaveBeenCalled();
  });

  it("markdown files get a preview toggle rendering the live model text", async () => {
    ReadRepoFile.mockResolvedValue("# Title\n");
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="docs/readme.md" onSaved={vi.fn()} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    mountCb?.({ getModifiedEditor: () => ({ getValue: () => "# Live Heading\n" }) });
    fireEvent.click(screen.getByRole("button", { name: "toggle markdown preview" }));
    expect(screen.getByRole("heading", { name: "Live Heading" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "toggle markdown preview" }));
    expect(screen.queryByRole("heading", { name: "Live Heading" })).not.toBeInTheDocument();
  });

  it("non-markdown files have no preview toggle", async () => {
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={vi.fn()} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "toggle markdown preview" })).not.toBeInTheDocument();
  });

  it("baseMode='head' diffs against HEAD (uncommitted only), not the merge-base", async () => {
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" baseMode="head" onSaved={vi.fn()} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());
    expect(ReadRepoFileHead).toHaveBeenCalledWith("/r/api", "src/a.ts");
    expect(ReadRepoFileBase).not.toHaveBeenCalled();
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
    ConfirmAction.mockResolvedValue(true);
    const onSaved = vi.fn();
    DiscardFile.mockResolvedValue(undefined);
    render(<EditorSettingsProvider><DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={onSaved} /></EditorSettingsProvider>);
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    fireEvent.click(await screen.findByRole("button", { name: /revert all changes/i }));

    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });
});
