import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("./Terminal", () => ({ Terminal: ({ cwd }: { cwd: string }) => <div data-testid="term">{cwd}</div> }));
vi.mock("../../../wailsjs/go/main/App", () => ({ TerminalWrite: vi.fn(), ListTerminalCommands: vi.fn().mockResolvedValue([]) }));

import { TerminalPanel } from "./TerminalPanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";

const Wrap = ({ children }: { children: React.ReactNode }) => (
  <HotkeyProvider platform="darwin">{children}</HotkeyProvider>
);

async function renderPanel() {
  render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
  await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
  // Give the terminal zone its scope — React's onFocus bubbles up to the root.
  fireEvent.focus(screen.getByTestId("term"));
}

beforeEach(() => vi.clearAllMocks());

describe("TerminalPanel hotkeys (layer)", () => {
  it("⌘T opens a new terminal (leader lapses to default)", async () => {
    await renderPanel();
    fireEvent.keyDown(window, { key: "t", metaKey: true }); // leader
    fireEvent.keyDown(window, { key: "x" }); // non-matching follow -> default terminal.new
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(2));
  });

  it("⌘T then H opens a terminal here (spawnCwd)", async () => {
    await renderPanel();
    fireEvent.keyDown(window, { key: "t", metaKey: true });
    fireEvent.keyDown(window, { key: "h" });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(2));
  });

  it("⌘T works without terminal focus (global scope)", async () => {
    // Deliberately do NOT focus the terminal zone. The commands are scope:"global",
    // so they must still fire — this is the bug where ⌘T did nothing unless the
    // terminal already held DOM focus.
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
    fireEvent.keyDown(window, { key: "t", metaKey: true });
    fireEvent.keyDown(window, { key: "h" });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(2));
  });

  it("⌘T then I opens the insert-command menu (state-driven popover)", async () => {
    await renderPanel();
    // Menu is closed until the hotkey opens it.
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    fireEvent.keyDown(window, { key: "t", metaKey: true });
    fireEvent.keyDown(window, { key: "i" });
    // A React popover — not a native <select> — so it works in WebKit, where
    // showPicker() is unavailable. Opening it does not depend on any native API.
    expect(screen.getByRole("menu")).toBeInTheDocument();
  });
});
