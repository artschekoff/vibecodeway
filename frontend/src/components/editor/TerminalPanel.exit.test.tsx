import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// A stub Terminal that exposes its onExit through a button, so the test can
// stand in for "the user typed exit" without a real PTY.
vi.mock("./Terminal", () => ({
  Terminal: ({ cwd, onExit }: { cwd: string; onExit?: () => void }) => (
    <div data-testid="term">
      {cwd}
      <button aria-label={`exit ${cwd}`} onClick={() => onExit?.()} />
    </div>
  ),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  TerminalWrite: vi.fn(), ListTerminalCommands: vi.fn().mockResolvedValue([]),
}));

import { TerminalPanel } from "./TerminalPanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";

const Wrap = ({ children }: { children: React.ReactNode }) => (
  <HotkeyProvider platform="darwin">{children}</HotkeyProvider>
);

beforeEach(() => vi.clearAllMocks());

describe("TerminalPanel — the shell exiting", () => {
  it("closes the tab when its shell ends", async () => {
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
    // A second terminal, so the first one closing leaves something behind.
    fireEvent.keyDown(window, { key: "t", metaKey: true });
    expect(screen.getAllByTestId("term")).toHaveLength(2);

    fireEvent.click(screen.getAllByLabelText("exit /r/api")[0]);
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
  });

  it("closes the last tab too, leaving an empty panel rather than respawning", async () => {
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));

    fireEvent.click(screen.getByLabelText("exit /r/api"));
    await waitFor(() => expect(screen.queryAllByTestId("term")).toHaveLength(0));
    // The seeding effect must not re-open one — that would make `exit` look broken.
    await waitFor(() => expect(screen.queryAllByTestId("term")).toHaveLength(0));
  });
});
