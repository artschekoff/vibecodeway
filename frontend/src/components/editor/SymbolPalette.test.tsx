import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { SymbolSearch, SymbolsIndex } = vi.hoisted(() => ({
  SymbolSearch: vi.fn(),
  SymbolsIndex: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ SymbolSearch, SymbolsIndex }));

import { SymbolPalette } from "./SymbolPalette";

beforeEach(() => {
  vi.clearAllMocks();
  SymbolsIndex.mockResolvedValue({ count: 2, tookMs: 5 });
  SymbolSearch.mockResolvedValue([
    { name: "getUser", kind: "function", path: "src/user.ts", language: "TypeScript", line: 12 },
    { name: "getUserById", kind: "function", path: "src/user.ts", language: "TypeScript", line: 40 },
  ]);
});

describe("SymbolPalette", () => {
  it("searches as you type and opens the arrow-selected symbol on Enter", async () => {
    const onPick = vi.fn();
    render(<SymbolPalette repoPath="/wt/api" open onClose={vi.fn()} onPick={onPick} />);

    fireEvent.change(screen.getByPlaceholderText(/search symbols/i), { target: { value: "getU" } });
    await waitFor(() => expect(SymbolSearch).toHaveBeenCalledWith("/wt/api", "getU", 50));
    await screen.findByText("getUser");

    fireEvent.keyDown(window, { key: "ArrowDown" });
    fireEvent.keyDown(window, { key: "Enter" });
    expect(onPick).toHaveBeenCalledWith("src/user.ts", 40);
  });

  it("shows the ctags install hint on the sentinel error", async () => {
    SymbolSearch.mockRejectedValue(new Error("ctags-not-found"));
    render(<SymbolPalette repoPath="/wt/api" open onClose={vi.fn()} onPick={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText(/search symbols/i), { target: { value: "x" } });
    expect(await screen.findByText(/brew install universal-ctags/i)).toBeInTheDocument();
  });

  it("clears the stale ctags hint on reopen", async () => {
    SymbolSearch.mockRejectedValue(new Error("ctags-not-found"));
    const { rerender } = render(<SymbolPalette repoPath="/wt/api" open onClose={vi.fn()} onPick={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText(/search symbols/i), { target: { value: "x" } });
    await screen.findByText(/brew install universal-ctags/i);

    rerender(<SymbolPalette repoPath="/wt/api" open={false} onClose={vi.fn()} onPick={vi.fn()} />);
    rerender(<SymbolPalette repoPath="/wt/api" open onClose={vi.fn()} onPick={vi.fn()} />);

    expect(screen.queryByText(/brew install universal-ctags/i)).not.toBeInTheDocument();
  });
});
