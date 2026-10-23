import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { AskAIChat, SummarizeAskChat, ListAgentTools, SaveAskChat, ListAskChats, GetAskChat, DeleteAskChat, ListTrackedRepos, ListRepoLocalPaths, ListLocalBranches, EnsureBranchWorktree } = vi.hoisted(() => ({
  ListTrackedRepos: vi.fn().mockResolvedValue([]),
  ListRepoLocalPaths: vi.fn().mockResolvedValue([]),
  ListLocalBranches: vi.fn().mockResolvedValue([]),
  EnsureBranchWorktree: vi.fn(),
  AskAIChat: vi.fn(),
  SummarizeAskChat: vi.fn(),
  ListAgentTools: vi.fn(),
  SaveAskChat: vi.fn().mockResolvedValue(undefined),
  ListAskChats: vi.fn().mockResolvedValue([]),
  GetAskChat: vi.fn(),
  DeleteAskChat: vi.fn().mockResolvedValue(undefined),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  AskAIChat,
  SummarizeAskChat,
  SaveAskChat,
  ListAskChats,
  GetAskChat,
  DeleteAskChat,
  ListAgentTools: ListAgentTools.mockResolvedValue([{ name: "search_contexts", description: "Full-text search across contexts." }]),
  ListOrganizations: vi.fn().mockResolvedValue([{ id: "o1", name: "acme" }]),
  ResolveToolConfirmation: vi.fn(),
  ListTrackedRepos,
  ListRepoLocalPaths,
  ListLocalBranches,
  EnsureBranchWorktree,
}));
vi.mock("../components/editor/TerminalPanel", () => ({
  TerminalPanel: ({ spawnCwd }: { spawnCwd: string }) => <div data-testid="term">{spawnCwd}</div>,
}));

// Capture the ask:progress handler so the test can drive live events.
const handlers: Record<string, (p: unknown) => void> = {};
const BrowserOpenURL = vi.hoisted(() => vi.fn());
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, cb: (p: unknown) => void) => { handlers[name] = cb; return () => {}; },
  EventsOff: () => {},
  BrowserOpenURL,
}));

// The page lives inside TabProvider in the app; tests stub the tab API.
const openNewContextTab = vi.hoisted(() => vi.fn());
vi.mock("../contexts/TabContext", () => ({
  useTabContextOptional: () => ({ openNewContextTab }),
}));

import AskAIPage from "./AskAIPage";

beforeEach(() => {
  AskAIChat.mockReset();
  SummarizeAskChat.mockReset();
  localStorage.clear();
  for (const k in handlers) delete handlers[k];
});

describe("AskAIPage persistence", () => {
  it("restores the chat log after unmount (tab close) and clears storage on Clear context", async () => {
    AskAIChat.mockResolvedValueOnce({ content: "the answer", steps: [] });
    const user = userEvent.setup();
    const { unmount } = render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "my question");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("the answer")).toBeInTheDocument();

    unmount();
    render(<AskAIPage />);
    expect(screen.getByText("my question")).toBeInTheDocument();
    expect(screen.getByText("the answer")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /clear context/i }));
    expect(screen.queryByText("the answer")).not.toBeInTheDocument();
    expect(JSON.parse(localStorage.getItem("vibecodeway:askai:log") ?? "[]")).toEqual([]);
  });
});

describe("AskAIPage live progress", () => {
  it("renders streamed tool calls while thinking", async () => {
    let resolve!: (v: unknown) => void;
    AskAIChat.mockImplementation(() => new Promise((r) => { resolve = r; }));
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "who closed tickets?");
    await user.click(screen.getByRole("button", { name: /thinking|ask/i }));

    // Backend streams a tool call before finishing.
    handlers["ask:progress"]?.({ phase: "tool", tool: "search_contexts", args: '{"query":"tickets"}' });
    expect(await screen.findByText(/search_contexts/)).toBeInTheDocument();

    // Now the answer arrives.
    resolve({ content: "answer", steps: [{ tool: "search_contexts", args: "{}", result: "[]" }] });
    expect(await screen.findByText("answer")).toBeInTheDocument();
  });

  it("shows the tooling reference", async () => {
    AskAIChat.mockResolvedValue({ content: "", steps: [] });
    render(<AskAIPage />);
    expect(await screen.findByText("search_contexts")).toBeInTheDocument();
    expect(screen.getByText(/Full-text search across contexts/)).toBeInTheDocument();
  });
});

describe("AskAIPage answer links", () => {
  it("opens a context:// link in a new context tab and an http link in the browser", async () => {
    AskAIChat.mockResolvedValueOnce({
      content: "See [referrals-v2](context://ede7a6fb-1234) and [JH-5598](https://jira.example.com/browse/JH-5598).",
      steps: [],
    });
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "which context?");
    await user.click(screen.getByRole("button", { name: /ask/i }));

    const ctxLink = await screen.findByRole("link", { name: "referrals-v2" });
    await user.click(ctxLink);
    expect(openNewContextTab).toHaveBeenCalledWith("ede7a6fb-1234", "referrals-v2");

    await user.click(screen.getByRole("link", { name: "JH-5598" }));
    expect(BrowserOpenURL).toHaveBeenCalledWith("https://jira.example.com/browse/JH-5598");
  });
});

describe("AskAIPage multi-turn chat log", () => {
  it("grows the log across turns and sends prior history on the second ask", async () => {
    AskAIChat
      .mockResolvedValueOnce({ content: "first answer", steps: [{ tool: "get_jira_issue", args: '{"key":"JH-1"}', result: '{"comment":"blocked by JH-2"}' }] })
      .mockResolvedValueOnce({ content: "second answer", steps: [] });
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "question one");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("first answer")).toBeInTheDocument();

    // First call had no prior history.
    expect(AskAIChat).toHaveBeenNthCalledWith(1, "[]", "question one", "");

    await user.type(screen.getByRole("textbox"), "question two");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("second answer")).toBeInTheDocument();

    // Second call's history carries the first user question and answer.
    const secondCallHistory = JSON.parse(AskAIChat.mock.calls[1][0]);
    // Tool results from the first turn are replayed so the model keeps what it read.
    expect(secondCallHistory).toEqual([
      { role: "user", content: "question one" },
      { role: "system", content: expect.stringContaining('## get_jira_issue {"key":"JH-1"}\n{"comment":"blocked by JH-2"}') },
      { role: "assistant", content: "first answer" },
    ]);
    expect(AskAIChat.mock.calls[1][1]).toBe("question two");

    // Log now shows 2 user + 2 assistant entries.
    expect(screen.getByText("question one")).toBeInTheDocument();
    expect(screen.getByText("question two")).toBeInTheDocument();
    expect(screen.getByText("first answer")).toBeInTheDocument();
    expect(screen.getByText("second answer")).toBeInTheDocument();
  });

  it("clears the log", async () => {
    AskAIChat.mockResolvedValueOnce({ content: "an answer", steps: [] });
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "a question");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("an answer")).toBeInTheDocument();

    const clearBtn = screen.getByRole("button", { name: /clear context/i });
    expect(clearBtn).toBeEnabled();
    await user.click(clearBtn);

    expect(screen.queryByText("a question")).not.toBeInTheDocument();
    expect(screen.queryByText("an answer")).not.toBeInTheDocument();
    expect(clearBtn).toBeDisabled();
  });

  it("disables clear and summarize when the log is empty", () => {
    render(<AskAIPage />);
    expect(screen.getByRole("button", { name: /clear context/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /summarize context/i })).toBeDisabled();
  });

  it("replaces the log with one summary entry", async () => {
    AskAIChat.mockResolvedValueOnce({ content: "an answer", steps: [] });
    SummarizeAskChat.mockResolvedValueOnce("compressed summary text");
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "a question");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("an answer")).toBeInTheDocument();

    const summarizeBtn = screen.getByRole("button", { name: /summarize context/i });
    await user.click(summarizeBtn);

    expect(await screen.findByText("Context summarized")).toBeInTheDocument();
    expect(await screen.findByText("compressed summary text")).toBeInTheDocument();
    expect(screen.queryByText("a question")).not.toBeInTheDocument();
    expect(screen.queryByText("an answer")).not.toBeInTheDocument();

    expect(SummarizeAskChat).toHaveBeenCalledWith(
      JSON.stringify([{ role: "user", content: "a question" }, { role: "assistant", content: "an answer" }])
    );

    // A follow-up ask sends the summary as a system turn.
    AskAIChat.mockResolvedValueOnce({ content: "follow-up answer", steps: [] });
    await user.type(screen.getByRole("textbox"), "follow up");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("follow-up answer")).toBeInTheDocument();
    const followUpHistory = JSON.parse(AskAIChat.mock.calls[1][0]);
    expect(followUpHistory).toEqual([
      { role: "system", content: "Summary of the previous conversation:\ncompressed summary text" },
    ]);
  });
});

describe("toHistory tool-result budget", () => {
  it("truncates long results and drops oldest tool blocks past the total budget", async () => {
    const { toHistory } = await import("./AskAIPage");
    const big = "x".repeat(50_000);
    const log = [
      { role: "user" as const, content: "q1" },
      { role: "assistant" as const, content: "a1", steps: [{ tool: "t", args: "{}", result: big }] },
      { role: "user" as const, content: "q2" },
      { role: "assistant" as const, content: "a2", steps: [{ tool: "t", args: "{}", result: big }] },
    ];
    const h = toHistory(log);
    const systems = h.filter((m) => m.role === "system");
    // Per-result cap: 8k + marker, so 50k never leaks; budget: 80k total still fits both 8k blocks.
    expect(systems).toHaveLength(2);
    for (const m of systems) {
      expect(m.content).toContain("…[truncated]");
      expect(m.content.length).toBeLessThan(9_000);
    }
    // Shrink: eleven 8k blocks exceed 80k, so the oldest is dropped (newest kept).
    const many = Array.from({ length: 11 }, (_, i) => [
      { role: "user" as const, content: `q${i}` },
      { role: "assistant" as const, content: `a${i}`, steps: [{ tool: "t", args: "{}", result: big }] },
    ]).flat();
    const h2 = toHistory(many);
    const kept = h2.filter((m) => m.role === "system").length;
    expect(kept).toBeGreaterThan(0);
    expect(kept).toBeLessThan(11);
    expect(h2[1].role).toBe("assistant"); // q0 → a0 with no tool block
    expect(h2[h2.length - 2].role).toBe("system"); // newest keeps its block
  });
});

describe("AskAIPage DB history", () => {
  it("saves the chat after an answer and reopens a past chat from History", async () => {
    SaveAskChat.mockClear();
    AskAIChat.mockResolvedValueOnce({ content: "ans", steps: [] });
    ListAskChats.mockResolvedValue([{ id: "old-1", title: "older question", createdAt: "2026-10-01T00:00:00Z", updatedAt: "2026-10-01T00:00:00Z" }]);
    GetAskChat.mockResolvedValue({ id: "old-1", title: "older question", logJson: JSON.stringify([
      { role: "user", content: "older question" },
      { role: "assistant", content: "older answer", steps: [] },
    ]), createdAt: "", updatedAt: "" });
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.type(screen.getByRole("textbox"), "fresh question");
    await user.click(screen.getByRole("button", { name: /ask/i }));
    expect(await screen.findByText("ans")).toBeInTheDocument();
    const last = SaveAskChat.mock.calls.at(-1)!;
    expect(last[1]).toBe("fresh question");
    expect(JSON.parse(last[2])).toHaveLength(2);
    const activeId = last[0];

    const picker = screen.getByLabelText("Chat history");
    await user.click(picker); // onFocus → ListAskChats
    expect(await screen.findByText(/older question/)).toBeInTheDocument();
    await user.selectOptions(picker, "old-1");
    expect(await screen.findByText("older answer")).toBeInTheDocument();
    expect(screen.queryByText("ans")).not.toBeInTheDocument();
    // Reopened chat keeps its own id when re-saved.
    expect(SaveAskChat.mock.calls.at(-1)![0]).toBe("old-1");
    expect(activeId).not.toBe("old-1");

    // Delete removes the open chat and starts an empty one.
    await user.click(screen.getByRole("button", { name: /delete chat/i }));
    expect(DeleteAskChat).toHaveBeenCalledWith("old-1");
    expect(screen.queryByText("older answer")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /delete chat/i })).toBeDisabled();
  });

  it("repo → branch picker opens an in-app terminal at the branch worktree", async () => {
    ListTrackedRepos.mockResolvedValue([{ id: "repo1", owner: "acme", name: "widgets" }]);
    ListRepoLocalPaths.mockResolvedValue([{ id: "p1", path: "/tmp/widgets" }]);
    ListLocalBranches.mockResolvedValue([{ name: "main", isCurrent: false }, { name: "feat/x", isCurrent: true }]);
    EnsureBranchWorktree.mockResolvedValue("/tmp/widgets/.worktrees/feat_x");
    const user = userEvent.setup();
    render(<AskAIPage />);

    await user.selectOptions(await screen.findByLabelText("Repository"), "repo1");
    const branchSel = (await screen.findByLabelText("Branch")) as HTMLSelectElement;
    await screen.findByRole("option", { name: "feat/x" });
    expect(branchSel.value).toBe("feat/x");

    await user.click(screen.getByRole("button", { name: /open agent terminal/i }));
    expect(EnsureBranchWorktree).toHaveBeenCalledWith("repo1", "feat/x");
    expect(await screen.findByTestId("term")).toHaveTextContent("/tmp/widgets/.worktrees/feat_x");
  });
});
