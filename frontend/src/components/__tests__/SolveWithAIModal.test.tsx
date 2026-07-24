import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { SolveWithAI } = vi.hoisted(() => ({
  SolveWithAI: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  SolveWithAI: SolveWithAI.mockResolvedValue("AGENT INSTRUCTION"),
}));

import { SolveWithAIModal } from "../SolveWithAIModal";

beforeEach(() => {
  vi.clearAllMocks();
  SolveWithAI.mockResolvedValue("AGENT INSTRUCTION");
});

describe("SolveWithAIModal", () => {
  it("defaults the task textarea to implement", () => {
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);
    expect(screen.getByRole("textbox")).toHaveValue("implement");
  });

  it("solves with AI and copies the returned instruction to the clipboard", async () => {
    const user = userEvent.setup();
    const writeTextSpy = vi.spyOn(navigator.clipboard, "writeText");
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /^solve$/i }));

    expect(SolveWithAI).toHaveBeenCalledWith("c1", "implement");
    expect(writeTextSpy).toHaveBeenCalledWith("AGENT INSTRUCTION");
    expect(
      await screen.findByText(/copied to clipboard/i)
    ).toBeInTheDocument();
  });

  it("surfaces the real error when Wails rejects with a plain string", async () => {
    const user = userEvent.setup();
    SolveWithAI.mockRejectedValueOnce("no repository attached to context");
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /^solve$/i }));

    expect(
      await screen.findByText("no repository attached to context")
    ).toBeInTheDocument();
  });

  it("opens the editor after a successful solve, but not on error", async () => {
    const user = userEvent.setup();
    const onOpenEditor = vi.fn();
    const { rerender } = render(
      <SolveWithAIModal contextId="c1" onClose={vi.fn()} onOpenEditor={onOpenEditor} />
    );
    await user.click(screen.getByRole("button", { name: /^solve$/i }));
    expect(await screen.findByText(/copied to clipboard/i)).toBeInTheDocument();
    expect(onOpenEditor).toHaveBeenCalledTimes(1);

    // Error path: onOpenEditor must NOT fire.
    onOpenEditor.mockClear();
    SolveWithAI.mockRejectedValueOnce("boom");
    rerender(<SolveWithAIModal contextId="c2" onClose={vi.fn()} onOpenEditor={onOpenEditor} />);
    await user.click(screen.getByRole("button", { name: /^solve$/i }));
    expect(await screen.findByText("boom")).toBeInTheDocument();
    expect(onOpenEditor).not.toHaveBeenCalled();
  });
});
