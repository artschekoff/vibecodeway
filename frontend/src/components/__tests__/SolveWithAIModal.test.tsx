import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { SolveWithAI, ListVCSBranches } = vi.hoisted(() => ({
  SolveWithAI: vi.fn(),
  ListVCSBranches: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ SolveWithAI, ListVCSBranches }));

import { SolveWithAIModal } from "../SolveWithAIModal";

const RESULT = { instruction: "AGENT INSTRUCTION", worktreePath: "/wt/api" };
const api = { repoId: "r1", branch: "feat/x", repo: { owner: "acme", name: "api" } };
const rdbms = { repoId: "r2", branch: "feat/y", repo: { owner: "acme", name: "rdbms" } };

beforeEach(() => {
  vi.clearAllMocks();
  SolveWithAI.mockResolvedValue(RESULT);
  ListVCSBranches.mockResolvedValue([api]);
});

describe("SolveWithAIModal", () => {
  it("defaults the task textarea to implement", () => {
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);
    expect(screen.getByRole("textbox")).toHaveValue("implement");
  });

  it("solves in the single linked branch and copies the instruction to the clipboard", async () => {
    const user = userEvent.setup();
    const writeTextSpy = vi.spyOn(navigator.clipboard, "writeText");
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);

    expect(await screen.findByText("Work in acme/api · feat/x")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /^solve$/i }));

    expect(SolveWithAI).toHaveBeenCalledWith("c1", "implement", "r1", "feat/x");
    expect(writeTextSpy).toHaveBeenCalledWith("AGENT INSTRUCTION");
    expect(
      await screen.findByText(/copied to clipboard/i)
    ).toBeInTheDocument();
  });

  it("asks which branch to work in when several are linked; only that one is solved", async () => {
    const user = userEvent.setup();
    ListVCSBranches.mockResolvedValue([api, rdbms]);
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);

    const select = await screen.findByRole("combobox", { name: /repository and branch/i });
    expect(screen.getByRole("button", { name: /^solve$/i })).toBeDisabled();
    await user.selectOptions(select, "r2\u0000feat/y");
    await user.click(screen.getByRole("button", { name: /^solve$/i }));

    expect(SolveWithAI).toHaveBeenCalledWith("c1", "implement", "r2", "feat/y");
  });

  it("surfaces the real error when Wails rejects with a plain string", async () => {
    const user = userEvent.setup();
    SolveWithAI.mockRejectedValueOnce("no repository attached to context");
    render(<SolveWithAIModal contextId="c1" onClose={vi.fn()} />);

    await screen.findByText(/work in/i);
    await user.click(screen.getByRole("button", { name: /^solve$/i }));

    expect(
      await screen.findByText("no repository attached to context")
    ).toBeInTheDocument();
  });

  it("opens the editor in the solved worktree, but not on error", async () => {
    const user = userEvent.setup();
    const onOpenEditor = vi.fn();
    const { rerender } = render(
      <SolveWithAIModal contextId="c1" onClose={vi.fn()} onOpenEditor={onOpenEditor} />
    );
    await screen.findByText(/work in/i);
    await user.click(screen.getByRole("button", { name: /^solve$/i }));
    expect(await screen.findByText(/copied to clipboard/i)).toBeInTheDocument();
    expect(onOpenEditor).toHaveBeenCalledWith("/wt/api");

    // Error path: onOpenEditor must NOT fire.
    onOpenEditor.mockClear();
    SolveWithAI.mockRejectedValueOnce("boom");
    rerender(<SolveWithAIModal contextId="c2" onClose={vi.fn()} onOpenEditor={onOpenEditor} />);
    await screen.findByText(/work in/i);
    await user.click(screen.getByRole("button", { name: /^solve$/i }));
    expect(await screen.findByText("boom")).toBeInTheDocument();
    expect(onOpenEditor).not.toHaveBeenCalled();
  });
});
