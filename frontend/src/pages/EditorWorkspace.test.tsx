import { useEffect } from "react";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Calls onSession with a stable fake id once mounted so TerminalPanel's
// internal sessionByKey map is populated — needed for the "ask author agent"
// terminal-insert test below, which asserts on TerminalWrite.
vi.mock("../components/editor/Terminal", () => ({
  Terminal: ({ cwd, onSession }: { cwd: string; onSession?: (id: string) => void }) => {
    useEffect(() => { onSession?.(`sess:${cwd}`); }, [cwd, onSession]);
    return <div data-testid="terminal">{cwd}</div>;
  },
}));
vi.mock("../components/editor/DiffEditorTab", () => ({
  DiffEditorTab: ({ path, highlightAnchor }: { path: string; highlightAnchor?: string }) => (
    <div data-testid="diff">{path}{highlightAnchor ? `:${highlightAnchor}` : ""}</div>
  ),
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
// TourCard has its own dedicated test suite (TourCard.test.tsx) — stubbed here
// so EditorWorkspace tests only exercise the Start/Resume-tour wiring, not
// TourCard's internals (which would pull in ReadRepoFile/Stage/Discard mocks).
vi.mock("../components/editor/TourCard", () => ({
  TourCard: ({ tour, onClose, onLocateAnchor, onAskAuthorAgent }: {
    tour: { title: string };
    onClose: () => void;
    onLocateAnchor: (repoPath: string, path: string, anchor: string) => void;
    onAskAuthorAgent: (text: string) => void;
  }) => (
    <div data-testid="tour-card">
      {tour.title}
      <button onClick={onClose}>close-tour</button>
      <button onClick={() => onLocateAnchor("/r/api", "src/a.ts", "anchor-x")}>locate-anchor</button>
      <button onClick={() => onAskAuthorAgent("Question about src/a.ts: why here?")}>ask-author-agent</button>
    </div>
  ),
}));

const { GetContextChanges, RefreshContext, RefreshContextPRs, GetEditorSettings, SetEditorSettings, ListTerminalCommands, TerminalWrite, AgentResumeCommand, ListEvents, AddUserEvent, DeleteUserEvent, StartCodeTour, GetCodeTour, SymbolsIndex } = vi.hoisted(() => ({
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
  StartCodeTour: vi.fn(),
  GetCodeTour: vi.fn(),
  SymbolsIndex: vi.fn().mockResolvedValue(undefined),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetContextChanges, RefreshContext, RefreshContextPRs, GetEditorSettings, SetEditorSettings, ListTerminalCommands, TerminalWrite, AgentResumeCommand, ListEvents, AddUserEvent, DeleteUserEvent, StartCodeTour, GetCodeTour, SymbolsIndex }));

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
  GetCodeTour.mockResolvedValue(null);
  StartCodeTour.mockResolvedValue(undefined);
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

  it("disables Start tour with a tooltip when the repo has no changes", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    const btn = await screen.findByRole("button", { name: "Start tour" });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute("title", "No changes to tour");
  });

  it("starts a tour and opens the tour card on success", async () => {
    GetContextChanges.mockResolvedValue([
      { repoName: "acme/api", repoPath: "/r/api", files: [{ path: "a.ts", status: "modified", workStatus: "modified" }] },
    ]);
    StartCodeTour.mockResolvedValue({ id: "t1", title: "Add feature", steps: [], stepIndex: 0, decisions: {}, repoPath: "/r/api" });
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    const btn = await screen.findByRole("button", { name: "Start tour" });
    await waitFor(() => expect(btn).not.toBeDisabled());
    fireEvent.click(btn);
    await waitFor(() => expect(StartCodeTour).toHaveBeenCalledWith("c1", "/r/api"));
    expect(await screen.findByTestId("tour-card")).toHaveTextContent("Add feature");
  });

  it("shows a quiet inline note for the stable 'no changes to tour' error", async () => {
    GetContextChanges.mockResolvedValue([
      { repoName: "acme/api", repoPath: "/r/api", files: [{ path: "a.ts", status: "modified", workStatus: "modified" }] },
    ]);
    StartCodeTour.mockRejectedValue(new Error("code tour: no changes to tour"));
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    const btn = await screen.findByRole("button", { name: "Start tour" });
    await waitFor(() => expect(btn).not.toBeDisabled());
    fireEvent.click(btn);
    await waitFor(() => expect(screen.getByText("No changes to tour")).toBeInTheDocument());
    expect(screen.queryByTestId("tour-card")).not.toBeInTheDocument();
  });

  it("clears the anchor highlight target when the tour card is closed", async () => {
    GetContextChanges.mockResolvedValue([
      { repoName: "acme/api", repoPath: "/r/api", files: [{ path: "a.ts", status: "modified", workStatus: "modified" }] },
    ]);
    StartCodeTour.mockResolvedValue({ id: "t1", title: "Add feature", steps: [], stepIndex: 0, decisions: {}, repoPath: "/r/api" });
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Open the file the tour will highlight, then start the tour.
    fireEvent.click(await screen.findByText("open-a"));
    await waitFor(() => expect(screen.getByTestId("diff")).toHaveTextContent("src/a.ts"));
    const startBtn = await screen.findByRole("button", { name: "Start tour" });
    await waitFor(() => expect(startBtn).not.toBeDisabled());
    fireEvent.click(startBtn);
    await screen.findByTestId("tour-card");

    // TourCard locates an anchor in the currently open file — DiffEditorTab
    // should receive it as highlightAnchor.
    fireEvent.click(screen.getByText("locate-anchor"));
    await waitFor(() => expect(screen.getByTestId("diff")).toHaveTextContent("src/a.ts:anchor-x"));

    // Closing the card must clear the highlight, not just hide the card —
    // otherwise the decoration/scroll persists on the still-open file.
    fireEvent.click(screen.getByText("close-tour"));
    await waitFor(() => expect(screen.queryByTestId("tour-card")).not.toBeInTheDocument());
    expect(screen.getByTestId("diff")).toHaveTextContent("src/a.ts");
    expect(screen.getByTestId("diff")).not.toHaveTextContent("anchor-x");
  });

  it("offers Resume tour when GetCodeTour finds an existing tour on mount", async () => {
    GetCodeTour.mockResolvedValue({ id: "t1", title: "Existing tour", steps: [], stepIndex: 0, decisions: {}, repoPath: "/r/api" });
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    const resumeBtn = await screen.findByRole("button", { name: "resume tour" });
    expect(screen.queryByTestId("tour-card")).not.toBeInTheDocument();
    fireEvent.click(resumeBtn);
    expect(await screen.findByTestId("tour-card")).toHaveTextContent("Existing tour");
  });

  it("wires TourCard's 'ask the author agent' escalation into the terminal insert mechanism", async () => {
    GetContextChanges.mockResolvedValue([
      { repoName: "acme/api", repoPath: "/r/api", files: [{ path: "a.ts", status: "modified", workStatus: "modified" }] },
    ]);
    StartCodeTour.mockResolvedValue({ id: "t1", title: "Add feature", steps: [], stepIndex: 0, decisions: {}, repoPath: "/r/api" });
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" />
      </EditorFilesProvider></HotkeyProvider>
    );
    // Wait for the seeded terminal's fake session to register before the tour starts.
    await waitFor(() => expect(screen.getByTestId("terminal")).toHaveTextContent("/r/api"));
    const startBtn = await screen.findByRole("button", { name: "Start tour" });
    await waitFor(() => expect(startBtn).not.toBeDisabled());
    fireEvent.click(startBtn);
    await screen.findByTestId("tour-card");

    fireEvent.click(screen.getByText("ask-author-agent"));
    await waitFor(() =>
      expect(TerminalWrite).toHaveBeenCalledWith(expect.any(String), "Question about src/a.ts: why here?"));
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

describe("EditorWorkspace sidebar notes", () => {
  // maxHeight caps the BOX but not the paint: without a clip, tall note content
  // renders straight over the repo selector and the Files/Git tabs below it.
  // Clipping also gives NotesPanel's inner overflowY:auto list a bounded height,
  // which is the only reason it can scroll at all.
  it("clips the notes pane so its content cannot paint over the controls below", async () => {
    render(
      <HotkeyProvider platform="darwin"><EditorFilesProvider>
        <EditorWorkspace contextId="c1" contextKey="JH-3940" />
      </EditorFilesProvider></HotkeyProvider>
    );
    await waitFor(() => expect(screen.getByTestId("terminal")).toBeInTheDocument());

    // Find the pane by the cap itself rather than by its content: the notes list
    // is the only thing in the sidebar with a max-height.
    const capped = Array.from(document.querySelectorAll<HTMLElement>("div"))
      .filter((d) => d.style.maxHeight);
    expect(capped).toHaveLength(1);
    expect(capped[0].style.overflow).toBe("hidden");
  });
});
