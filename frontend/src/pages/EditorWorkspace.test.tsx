import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../components/editor/Terminal", () => ({ Terminal: ({ cwd }: { cwd: string }) => <div data-testid="terminal">{cwd}</div> }));
vi.mock("../components/editor/DiffEditorTab", () => ({
  DiffEditorTab: ({ path }: { path: string }) => <div data-testid="diff">{path}</div>,
}));
vi.mock("../components/editor/ChangedFilesPanel", () => ({
  ChangedFilesPanel: ({ onOpenFile, onRefresh }: { onOpenFile: (rp: string, rn: string, p: string, s: string) => void; onRefresh?: () => void }) => (
    <>
      <button onClick={() => onOpenFile("/r/api", "acme/api", "src/a.ts", "modified")}>open-a</button>
      {onRefresh && <button aria-label="refresh from github" onClick={onRefresh}>refresh</button>}
    </>
  ),
}));
vi.mock("../components/editor/EditorPRInfo", () => ({
  EditorPRInfo: ({ contextId, activeRepoId }: { contextId: string; activeRepoId?: string }) => (
    <div data-testid="pr-info">{contextId}:{activeRepoId ?? "none"}</div>
  ),
}));

const { GetContextChanges, RefreshContext, RefreshContextPRs, GetEditorSettings, SetEditorSettings, ListTerminalCommands, TerminalWrite, AgentResumeCommand, ListEvents, AddUserEvent, DeleteUserEvent } = vi.hoisted(() => ({
  GetContextChanges: vi.fn(),
  RefreshContext: vi.fn(),
  RefreshContextPRs: vi.fn(),
  GetEditorSettings: vi.fn(),
  SetEditorSettings: vi.fn(),
  ListTerminalCommands: vi.fn().mockResolvedValue([]),
  TerminalWrite: vi.fn(),
  AgentResumeCommand: vi.fn().mockResolvedValue(""),
  ListEvents: vi.fn().mockResolvedValue([]),
  AddUserEvent: vi.fn(),
  DeleteUserEvent: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetContextChanges, RefreshContext, RefreshContextPRs, GetEditorSettings, SetEditorSettings, ListTerminalCommands, TerminalWrite, AgentResumeCommand, ListEvents, AddUserEvent, DeleteUserEvent }));

import { EditorWorkspace } from "./EditorWorkspace";
import { EditorFilesProvider } from "../contexts/EditorFilesContext";
import { HotkeyProvider } from "../hotkeys/HotkeyProvider";

beforeEach(() => {
  vi.clearAllMocks();
  // TerminalPanel persists tabs under persistKey={contextId}, and every test here
  // uses contextId="c1" — without clearing, an earlier test's saved layout leaks
  // into the next one (e.g. masking initialCwd).
  localStorage.clear();
  GetContextChanges.mockResolvedValue([{ repoName: "acme/api", repoPath: "/r/api", files: [] }]);
  RefreshContext.mockResolvedValue({});
  GetEditorSettings.mockResolvedValue("");
  SetEditorSettings.mockResolvedValue(undefined);
});

describe("EditorWorkspace", () => {
  it("starts terminal at first repo and opens a diff tab on file click", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));

    fireEvent.click(screen.getByText("open-a"));
    await waitFor(() => expect(screen.getByTestId("diff")).toHaveTextContent("src/a.ts"));
  });

  it("shows the context key and Context Editing mode in the header", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="JH-3940" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByText("JH-3940")).toBeInTheDocument());
    expect(screen.getByText("Context Editing")).toBeInTheDocument();
  });

  it("copies the context name to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="JH-3940" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByText("JH-3940")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "copy context name" }));
    expect(writeText).toHaveBeenCalledWith("JH-3940");
  });

  it("shows PR Review mode for a PR-review context key", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="juna-health/juna-api#4521" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByText("juna-health/juna-api#4521")).toBeInTheDocument());
    expect(screen.getByText("PR Review")).toBeInTheDocument();
  });

  it("renders the PR info block for the context", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="juna-health/juna-api#349" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("pr-info")).toHaveTextContent("c1"));
  });

  it("root fills its flex container instead of shrink-wrapping to content", async () => {
    // TabContent mounts each editor tab inside a `display:flex` wrapper. Without
    // `flex:1` on our root it becomes a flex item with flex-grow:0 and shrink-
    // wraps to content width — collapsing the diff pane and letting the sidebar
    // take over the width (regression from the keep-tabs-mounted change).
    const { container } = render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toBeInTheDocument());
    const style = (container.firstChild as HTMLElement).getAttribute("style") ?? "";
    expect(style).toContain("flex: 1");
    expect(style).toContain("min-width: 0");
  });

  it("refresh button re-asks GitHub then re-fetches the context", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));
    GetContextChanges.mockClear();

    // The refresh icon lives in the changed-files repo header, which loads async.
    fireEvent.click(await screen.findByRole("button", { name: /refresh from github/i }));

    await waitFor(() => expect(RefreshContext).toHaveBeenCalledWith("c1"));
    await waitFor(() => expect(GetContextChanges).toHaveBeenCalled());
  });

  it("opens the terminal at initialCwd when provided", async () => {
    GetContextChanges.mockResolvedValue([
      { repoId: "id-api", repoName: "acme/api", repoPath: "/r/api", files: [] },
      { repoId: "id-rdbms", repoName: "acme/rdbms", repoPath: "/r/rdbms", files: [] },
    ]);
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" initialCwd="/r/rdbms" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Terminal mock renders its cwd as text.
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/rdbms"));
  });

  it("does not repo-filter the PR head in a single-repo context (PR review mode)", async () => {
    // A PR-review context has exactly one repo and one PR. Narrowing the PR head
    // by activeRepoId is meaningless here and, if the derived id fails to match
    // the PR's repoId for any reason, it would hide the sole PR entirely
    // (regression). With one repo we must pass no activeRepoId so the head always
    // shows.
    GetContextChanges.mockResolvedValue([
      { repoId: "id-api", repoName: "acme/api", repoPath: "/r/api", files: [] },
    ]);
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Wait until the repo has loaded (terminal is rooted at its path) so we assert
    // the resolved state, not the transient empty-cwd first render.
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));
    expect(screen.getByTestId("pr-info")).toHaveTextContent("c1:none");
  });

  it("passes the selected repo's id to the PR head and updates it when the selector changes", async () => {
    GetContextChanges.mockResolvedValue([
      { repoId: "id-api", repoName: "acme/api", repoPath: "/r/api", files: [] },
      { repoId: "id-rdbms", repoName: "acme/rdbms", repoPath: "/r/rdbms", files: [] },
    ]);
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Defaults to the first repo's id.
    await waitFor(() => expect(screen.getByTestId("pr-info")).toHaveTextContent("c1:id-api"));

    // Switching the repo selector re-targets the PR head at the new repo. Scope
    // to the repo <select> (the terminal command selector is also a combobox).
    const repoSelect = screen.getAllByRole("combobox").find((el) => el.querySelector('option[value="/r/rdbms"]'))!;
    fireEvent.change(repoSelect, { target: { value: "/r/rdbms" } });
    await waitFor(() => expect(screen.getByTestId("pr-info")).toHaveTextContent("c1:id-rdbms"));
  });

  it("shows an always-visible refresh-PRs button that re-syncs PRs on click", async () => {
    RefreshContextPRs.mockResolvedValue([]);
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Button is present even though EditorPRInfo (mocked) shows no PRs.
    const btn = await screen.findByRole("button", { name: /refresh prs/i });
    fireEvent.click(btn);
    await waitFor(() => expect(RefreshContextPRs).toHaveBeenCalledWith("c1"));
  });

  it("surfaces a PR-refresh error inline instead of failing silently", async () => {
    RefreshContextPRs.mockRejectedValue(new Error("gh auth failed"));
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    fireEvent.click(await screen.findByRole("button", { name: /refresh prs/i }));
    await waitFor(() => expect(screen.getByText("gh auth failed")).toBeInTheDocument());
  });
  // ⌘T (and every other terminal hotkey) is registered by TerminalPanel, so the
  // panel must be mounted even before GetContextChanges resolves and even for a
  // context whose repos aren't on disk — otherwise the key silently does nothing.
  it("keeps the terminal hotkeys alive before repo paths load", async () => {
    let resolveChanges: (v: unknown) => void = () => {};
    GetContextChanges.mockReturnValue(new Promise((r) => { resolveChanges = r; }));
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Panel mounted (so its ⌘T handlers are registered) before any repo path exists.
    expect(screen.getByRole("button", { name: /new terminal/i })).toBeInTheDocument();
    resolveChanges([{ repoName: "acme/api", repoPath: "/r/api", files: [] }]);
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));
    fireEvent.keyDown(window, { key: "t", metaKey: true });
    await waitFor(() => expect(screen.getAllByTestId("terminal")).toHaveLength(2));
  });

  it("keeps the terminal hotkeys alive for a context with no repo path on disk", async () => {
    GetContextChanges.mockResolvedValue([{ repoName: "acme/api", repoPath: "", files: [] }]);
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    expect(await screen.findByText(/no repo path on disk/i)).toBeInTheDocument();
    // Panel still mounted, so the terminal hotkeys stay registered.
    expect(screen.getByRole("button", { name: /new terminal/i })).toBeInTheDocument();
  });
});

// Walks up from the mocked terminal to the pane that actually carries the
// height, without depending on how deeply TerminalPanel nests its own markup.
function terminalPane(): HTMLElement {
  let el: HTMLElement | null = screen.getByTestId("terminal");
  while (el && !/^\d+px$/.test(el.style.height)) el = el.parentElement;
  if (!el) throw new Error("no ancestor of the terminal carries a px height");
  return el;
}

describe("EditorWorkspace terminal resize", () => {
  // The handle registers its mousemove listener once on mousedown and emits
  // INCREMENTAL deltas, so each step must accumulate onto the previous height.
  // Computing from a value captured at render time freezes it for the whole
  // drag: the panel jitters by one step and never actually moves.
  it("accumulates incremental drag steps instead of restarting from the same height", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));

    const pane = terminalPane();
    const startHeight = Number.parseInt(pane.style.height, 10);
    expect(startHeight).toBe(260); // TERMINAL_DEFAULT

    // Drag the handle DOWN in four 20px steps — the terminal should shrink 80px.
    const handle = screen.getByRole("separator", { name: "resize terminal" });
    fireEvent.mouseDown(handle, { clientY: 400 });
    fireEvent.mouseMove(window, { clientY: 420 });
    fireEvent.mouseMove(window, { clientY: 440 });
    fireEvent.mouseMove(window, { clientY: 460 });
    fireEvent.mouseMove(window, { clientY: 480 });
    fireEvent.mouseUp(window);

    expect(Number.parseInt(pane.style.height, 10)).toBe(startHeight - 80);
  });

  it("grows the terminal when dragged up, accumulating the same way", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));

    const pane = terminalPane();
    const handle = screen.getByRole("separator", { name: "resize terminal" });

    fireEvent.mouseDown(handle, { clientY: 400 });
    fireEvent.mouseMove(window, { clientY: 370 });
    fireEvent.mouseMove(window, { clientY: 340 });
    fireEvent.mouseUp(window);

    expect(Number.parseInt(pane.style.height, 10)).toBe(320); // 260 + 30 + 30
  });
});
