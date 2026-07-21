import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { AskAI, ListAgentTools } = vi.hoisted(() => ({ AskAI: vi.fn(), ListAgentTools: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({
  AskAI,
  ListAgentTools: ListAgentTools.mockResolvedValue([{ name: "search_contexts", description: "Full-text search across contexts." }]),
  ListOrganizations: vi.fn().mockResolvedValue([{ id: "o1", name: "jiro" }]),
}));

// Capture the ask:progress handler so the test can drive live events.
const handlers: Record<string, (p: unknown) => void> = {};
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, cb: (p: unknown) => void) => { handlers[name] = cb; return () => {}; },
  EventsOff: () => {},
}));

import AskAIPage from "./AskAIPage";

beforeEach(() => { AskAI.mockReset(); for (const k in handlers) delete handlers[k]; });

describe("AskAIPage live progress", () => {
  it("renders streamed tool calls while thinking", async () => {
    let resolve!: (v: unknown) => void;
    AskAI.mockImplementation(() => new Promise((r) => { resolve = r; }));
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
    AskAI.mockResolvedValue({ content: "", steps: [] });
    render(<AskAIPage />);
    expect(await screen.findByText("search_contexts")).toBeInTheDocument();
    expect(screen.getByText(/Full-text search across contexts/)).toBeInTheDocument();
  });
});
