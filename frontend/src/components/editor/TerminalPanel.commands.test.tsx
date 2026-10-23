import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock Terminal: reports a fake session id, prints cwd.
vi.mock("./Terminal", () => ({
  Terminal: ({ cwd, onSession }: { cwd: string; onSession?: (id: string) => void }) => {
    onSession?.("sess-1");
    return <div data-testid="term">{cwd}</div>;
  },
}));

const { TerminalWrite, ListTerminalCommands } = vi.hoisted(() => ({
  TerminalWrite: vi.fn(),
  ListTerminalCommands: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ TerminalWrite, ListTerminalCommands }));

import { TerminalPanel, tabLabel } from "./TerminalPanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";

const Wrap = ({ children }: { children: React.ReactNode }) => (
  <HotkeyProvider platform="darwin">{children}</HotkeyProvider>
);

describe("tabLabel", () => {
  it("prepends the repo name for a worktree path", () => {
    expect(tabLabel("/Users/a/work/acme-api/.worktrees/feat_JH-4302")).toBe("acme-api / feat_JH-4302");
    expect(tabLabel("/x/acme-functions/.worktrees/feat_JH-4836")).toBe("acme-functions / feat_JH-4836");
  });
  it("shows the name once for a plain checkout (repo === folder)", () => {
    expect(tabLabel("/Users/a/work/acme-api")).toBe("acme-api");
  });
});

beforeEach(() => {
  vi.clearAllMocks();
  TerminalWrite.mockResolvedValue(undefined);
  ListTerminalCommands.mockResolvedValue([
    { id: "c1", title: "Inject", command: "/load-context <context-name>", sendEnter: false, createdAt: "", updatedAt: "" },
    { id: "c2", title: "Run", command: "npm test", sendEnter: true, createdAt: "", updatedAt: "" },
  ]);
});

describe("TerminalPanel command selector", () => {
  it("substitutes <context-name> and types the command without Enter", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="tell-us-more-fin" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    // Open the insert-command popover, then pick the command by title.
    fireEvent.click(await screen.findByLabelText("Insert terminal command"));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Inject" }));
    await waitFor(() =>
      expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "/load-context tell-us-more-fin")
    );
  });

  it("appends Enter (\\r) when the command has sendEnter", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="ctx" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    fireEvent.click(await screen.findByLabelText("Insert terminal command"));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Run" }));
    await waitFor(() => expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "npm test\r"));
  });

  it("navigates the menu with ArrowDown and selects with Enter", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="ctx" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    fireEvent.click(await screen.findByLabelText("Insert terminal command"));
    await screen.findAllByRole("menuitem");
    // ArrowDown moves the active item from Inject (0) to Run (1); Enter selects it.
    fireEvent.keyDown(window, { key: "ArrowDown" });
    fireEvent.keyDown(window, { key: "Enter" });
    await waitFor(() => expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "npm test\r"));
  });

  it("Enter with no arrow selects the first item", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="tell-us-more-fin" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    fireEvent.click(await screen.findByLabelText("Insert terminal command"));
    await screen.findAllByRole("menuitem");
    fireEvent.keyDown(window, { key: "Enter" });
    await waitFor(() =>
      expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "/load-context tell-us-more-fin")
    );
  });
});
