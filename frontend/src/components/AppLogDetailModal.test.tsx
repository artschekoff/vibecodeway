import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { AppLogDetailModal } from "./AppLogDetailModal";

const entry = {
  id: 1, level: "info", ts: 1.5, caller: "openai/client.go:200",
  msg: "llm.chat_completion model=gpt-4o tokens=10→5 (total 15) in 2.1s",
  createdAt: "2026-08-11T00:00:00Z",
  fields: JSON.stringify({ request: '[{"role":"user","content":"hi"}]', durationMs: 2100, model: "gpt-4o" }),
} as any;

describe("AppLogDetailModal", () => {
  it("shows the message, caller, and the full request text", () => {
    render(<AppLogDetailModal entry={entry} onClose={vi.fn()} />);
    expect(screen.getByText(/llm.chat_completion/)).toBeInTheDocument();
    expect(screen.getByText(/openai\/client\.go:200/)).toBeInTheDocument();
    expect(screen.getByText(/"role":"user"/)).toBeInTheDocument();
  });

  it("closes on Escape", async () => {
    const onClose = vi.fn();
    render(<AppLogDetailModal entry={entry} onClose={onClose} />);
    await userEvent.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });
});
