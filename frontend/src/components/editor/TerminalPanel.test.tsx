import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Mock the real Terminal (xterm + Wails IPC) with a leaf that just prints its cwd.
vi.mock("./Terminal", () => ({ Terminal: ({ cwd }: { cwd: string }) => <div data-testid="term">{cwd}</div> }));

vi.mock("../../../wailsjs/go/main/App", () => ({
  TerminalWrite: vi.fn(),
  ListTerminalCommands: vi.fn().mockResolvedValue([]),
  AgentResumeCommand: vi.fn().mockResolvedValue(""),
}));

import { TerminalPanel } from "./TerminalPanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";

const Wrap = ({ children }: { children: React.ReactNode }) => (
  <HotkeyProvider platform="darwin">{children}</HotkeyProvider>
);

describe("TerminalPanel", () => {
  it("auto-creates one terminal at spawnCwd", async () => {
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
    expect(screen.getByTestId("term")).toHaveTextContent("/r/api");
  });

  it("+ opens a new terminal in the active terminal's folder and keeps both mounted", async () => {
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));

    fireEvent.click(screen.getByRole("button", { name: "new terminal" }));

    const terms = screen.getAllByTestId("term");
    expect(terms).toHaveLength(2); // both stay mounted
    expect(terms.every((t) => t.textContent === "/r/api")).toBe(true);
  });

  it("existing terminals keep their folder when spawnCwd changes; new + uses active cwd", async () => {
    const { rerender } = render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));

    // Repo selector changes the spawn folder for future terminals only.
    rerender(<TerminalPanel spawnCwd="/r/other" />);
    expect(screen.getByTestId("term")).toHaveTextContent("/r/api");

    fireEvent.click(screen.getByRole("button", { name: "new terminal" }));
    const terms = screen.getAllByTestId("term");
    expect(terms).toHaveLength(2);
    expect(terms.every((t) => t.textContent === "/r/api")).toBe(true); // active cwd, not /r/other
  });

  it("× removes a terminal", async () => {
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
    fireEvent.click(screen.getByRole("button", { name: "new terminal" }));
    expect(screen.getAllByTestId("term")).toHaveLength(2);

    fireEvent.click(screen.getAllByRole("button", { name: /close terminal/i })[0]);
    expect(screen.getAllByTestId("term")).toHaveLength(1);
  });
});
