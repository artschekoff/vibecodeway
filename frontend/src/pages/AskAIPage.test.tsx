import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { AskAIChat, SummarizeAskChat, ListAgentTools } = vi.hoisted(() => ({
  AskAIChat: vi.fn(),
  SummarizeAskChat: vi.fn(),
  ListAgentTools: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  AskAIChat,
  SummarizeAskChat,
  ListAgentTools: ListAgentTools.mockResolvedValue([{ name: "search_contexts", description: "Full-text search across contexts." }]),
  ListOrganizations: vi.fn().mockResolvedValue([{ id: "o1", name: "jiro" }]),
  ResolveToolConfirmation: vi.fn(),
}));

// Capture the ask:progress handler so the test can drive live events.
const handlers: Record<string, (p: unknown) => void> = {};
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, cb: (p: unknown) => void) => { handlers[name] = cb; return () => {}; },
  EventsOff: () => {},
}));

import AskAIPage from "./AskAIPage";

beforeEach(() => {
  AskAIChat.mockReset();
  SummarizeAskChat.mockReset();
  for (const k in handlers) delete handlers[k];
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

describe("AskAIPage multi-turn chat log", () => {
  it("grows the log across turns and sends prior history on the second ask", async () => {
    AskAIChat
      .mockResolvedValueOnce({ content: "first answer", steps: [] })
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
    expect(secondCallHistory).toEqual([
      { role: "user", content: "question one" },
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
