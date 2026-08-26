import { render, screen, waitFor, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../../lib/monacoSetup", () => ({ setupMonaco: vi.fn() }));

const { initVimMode, dispose } = vi.hoisted(() => {
  const dispose = vi.fn();
  return { initVimMode: vi.fn((_editor: unknown, _status: HTMLElement) => ({ dispose })), dispose };
});
vi.mock("monaco-vim", () => ({ initVimMode }));

let mountCb: ((editor: unknown, monaco: unknown) => void) | undefined;
vi.mock("@monaco-editor/react", () => ({
  DiffEditor: (props: { onMount?: (e: unknown, m: unknown) => void }) => {
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
import { setVimMode } from "../../lib/vimMode";

const modifiedEditor = { id: "modified", onDidChangeModelContent: vi.fn(), getValue: () => "new\n" };
const diffEditor = { getModifiedEditor: () => modifiedEditor, getLineChanges: () => [] };

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  mountCb = undefined;
  ReadRepoFileBase.mockResolvedValue("old\n");
  ReadRepoFileHead.mockResolvedValue("head\n");
  ReadRepoFile.mockResolvedValue("new\n");
  GetEditorSettings.mockResolvedValue("");
  SetEditorSettings.mockResolvedValue(undefined);
  FileBlame.mockResolvedValue([]);
});

const renderTab = () =>
  render(
    <EditorSettingsProvider>
      <DiffEditorTab repoPath="/repo" path="a.ts" baseMode="head" onSaved={() => {}} />
    </EditorSettingsProvider>,
  );

describe("DiffEditorTab vim mode", () => {
  it("binds the shared vim layer to the modified (editable) side", async () => {
    setVimMode(true);
    renderTab();
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    act(() => { mountCb?.(diffEditor, {}); });

    await waitFor(() => expect(initVimMode).toHaveBeenCalled());
    expect(initVimMode.mock.calls[0][0]).toBe(modifiedEditor);
    expect(initVimMode.mock.calls[0][1]).toBe(screen.getByTestId("vim-status"));
  });

  it("leaves the code editor unbound when vim mode is off", async () => {
    renderTab();
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());

    act(() => { mountCb?.(diffEditor, {}); });

    await waitFor(() => expect(ReadRepoFile).toHaveBeenCalled());
    expect(initVimMode).not.toHaveBeenCalled();
  });

  it("binds and unbinds as the app-wide setting is toggled", async () => {
    renderTab();
    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());
    act(() => { mountCb?.(diffEditor, {}); });

    act(() => { setVimMode(true); });
    await waitFor(() => expect(initVimMode).toHaveBeenCalledTimes(1));

    act(() => { setVimMode(false); });
    await waitFor(() => expect(dispose).toHaveBeenCalledTimes(1));
  });
});
