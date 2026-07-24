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

const { ReadRepoFile, ReadRepoFileHead, SaveRepoFile } = vi.hoisted(() => ({
  ReadRepoFile: vi.fn(), ReadRepoFileHead: vi.fn(), SaveRepoFile: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ ReadRepoFile, ReadRepoFileHead, SaveRepoFile }));

import { DiffEditorTab } from "./DiffEditorTab";

beforeEach(() => {
  vi.clearAllMocks();
  ReadRepoFileHead.mockResolvedValue("old\n");
  ReadRepoFile.mockResolvedValue("new\n");
  SaveRepoFile.mockResolvedValue(undefined);
});

describe("DiffEditorTab", () => {
  it("loads head+working and saves the modified model value", async () => {
    const onSaved = vi.fn();
    render(<DiffEditorTab repoPath="/r/api" path="src/a.ts" onSaved={onSaved} />);

    await waitFor(() => expect(screen.getByTestId("diff-editor")).toBeInTheDocument());
    expect(ReadRepoFileHead).toHaveBeenCalledWith("/r/api", "src/a.ts");
    expect(ReadRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts");

    // Simulate Monaco mounting with an editor whose modified model has edits.
    mountCb?.({ getModifiedEditor: () => ({ getValue: () => "edited\n" }) });

    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    await waitFor(() => expect(SaveRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts", "edited\n"));
    expect(onSaved).toHaveBeenCalled();
  });
});
