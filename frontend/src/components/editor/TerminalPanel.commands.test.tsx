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
    expect(tabLabel("/Users/a/jiro/juna-api/.worktrees/feat_JH-4302")).toBe("juna-api / feat_JH-4302");
    expect(tabLabel("/x/juna-functions/.worktrees/feat_JH-4836")).toBe("juna-functions / feat_JH-4836");
  });
  it("shows the name once for a plain checkout (repo === folder)", () => {
    expect(tabLabel("/Users/a/jiro/juna-api")).toBe("juna-api");
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
    const select = await screen.findByLabelText("Insert terminal command");
    fireEvent.change(select, { target: { value: "c1" } });
    await waitFor(() =>
      expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "/load-context tell-us-more-fin")
    );
  });

  it("appends Enter (\\r) when the command has sendEnter", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="ctx" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    const select = await screen.findByLabelText("Insert terminal command");
    fireEvent.change(select, { target: { value: "c2" } });
    await waitFor(() => expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "npm test\r"));
  });
});
