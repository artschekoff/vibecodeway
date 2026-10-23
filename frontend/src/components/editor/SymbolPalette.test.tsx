import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const { SymbolSearch, SymbolsIndex, GetRepoFileTree } = vi.hoisted(() => ({
  SymbolSearch: vi.fn(),
  SymbolsIndex: vi.fn(),
  GetRepoFileTree: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ SymbolSearch, SymbolsIndex, GetRepoFileTree }));

import { SymbolPalette, filterFiles } from "./SymbolPalette";

beforeEach(() => {
  vi.clearAllMocks();
  SymbolsIndex.mockResolvedValue({ count: 2, tookMs: 5 });
  SymbolSearch.mockResolvedValue([
    { name: "getUser", kind: "function", path: "src/user.ts", language: "TypeScript", line: 12 },
    { name: "getUserById", kind: "function", path: "src/user.ts", language: "TypeScript", line: 40 },
  ]);
  GetRepoFileTree.mockResolvedValue(["src/user.ts", "src/models/user.dto.ts", "README.md"]);
});

describe("filterFiles", () => {
  it("ranks basename prefix over basename substring over path substring", () => {
    const paths = ["a/buser.ts", "b/user.ts", "user-api/readme.md"];
    expect(filterFiles(paths, "user", 10)).toEqual(["b/user.ts", "a/buser.ts", "user-api/readme.md"]);
  });
  it("returns nothing for an empty query", () => {
    expect(filterFiles(["a.ts"], "", 10)).toEqual([]);
  });
});

describe("SymbolPalette", () => {
  it("'#' query searches symbols and opens the arrow-selected one on Enter", async () => {
    const onPick = vi.fn();
    render(<SymbolPalette repoPath="/wt/api" open initialQuery="#" onClose={vi.fn()} onPick={onPick} />);

    fireEvent.change(screen.getByPlaceholderText(/search symbols/i), { target: { value: "#getU" } });
    await waitFor(() => expect(SymbolSearch).toHaveBeenCalledWith("/wt/api", "getU", 50));
    await screen.findByText("getUser");

    fireEvent.keyDown(window, { key: "ArrowDown" });
    fireEvent.keyDown(window, { key: "Enter" });
    expect(onPick).toHaveBeenCalledWith("src/user.ts", 40);
  });

  it("plain query searches file names only and opens the file at line 1", async () => {
    const onPick = vi.fn();
    render(<SymbolPalette repoPath="/wt/api" open onClose={vi.fn()} onPick={onPick} />);

    fireEvent.change(screen.getByPlaceholderText(/search files/i), { target: { value: "user" } });
    await screen.findByText("user.ts");
    expect(SymbolSearch).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: "Enter" });
    expect(onPick).toHaveBeenCalledWith("src/user.ts", 1);
  });

  it("'*' query searches files and symbols together", async () => {
    render(<SymbolPalette repoPath="/wt/api" open initialQuery="*" onClose={vi.fn()} onPick={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText(/search files and symbols/i), { target: { value: "*user" } });
    await screen.findByText("getUser");
    expect(screen.getByText("user.ts")).toBeInTheDocument();
    await waitFor(() => expect(SymbolSearch).toHaveBeenCalledWith("/wt/api", "user", 50));
  });

  it("shows the ctags install hint on the sentinel error", async () => {
    SymbolSearch.mockRejectedValue(new Error("ctags-not-found"));
    render(<SymbolPalette repoPath="/wt/api" open initialQuery="#" onClose={vi.fn()} onPick={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText(/search symbols/i), { target: { value: "#x" } });
    expect(await screen.findByText(/brew install universal-ctags/i)).toBeInTheDocument();
  });

  it("clears the stale ctags hint on reopen", async () => {
    SymbolSearch.mockRejectedValue(new Error("ctags-not-found"));
    const { rerender } = render(<SymbolPalette repoPath="/wt/api" open initialQuery="#" onClose={vi.fn()} onPick={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText(/search symbols/i), { target: { value: "#x" } });
    await screen.findByText(/brew install universal-ctags/i);

    rerender(<SymbolPalette repoPath="/wt/api" open={false} initialQuery="#" onClose={vi.fn()} onPick={vi.fn()} />);
    rerender(<SymbolPalette repoPath="/wt/api" open initialQuery="#" onClose={vi.fn()} onPick={vi.fn()} />);

    expect(screen.queryByText(/brew install universal-ctags/i)).not.toBeInTheDocument();
  });
});
