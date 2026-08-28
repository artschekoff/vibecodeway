import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("./Terminal", () => ({ Terminal: ({ cwd }: { cwd: string }) => <div data-testid="term">{cwd}</div> }));

const H = vi.hoisted(() => ({
  TerminalWrite: vi.fn(),
  ListTerminalCommands: vi.fn().mockResolvedValue([]),
  AgentResumeCommand: vi.fn().mockResolvedValue(""),
}));
vi.mock("../../../wailsjs/go/main/App", () => H);

import { TerminalPanel } from "./TerminalPanel";
import { HotkeyProvider } from "../../hotkeys/HotkeyProvider";
import { loadTerminals, saveTerminals } from "../../lib/terminalSession";

const Wrap = ({ children }: { children: React.ReactNode }) => (
  <HotkeyProvider platform="darwin">{children}</HotkeyProvider>
);

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
});

describe("TerminalPanel — layout persistence", () => {
  it("restores the saved tabs instead of seeding one", async () => {
    saveTerminals("ctx1", [
      { cwd: "/r/api", openedAt: 1 },
      { cwd: "/r/web", openedAt: 2 },
    ]);

    render(<TerminalPanel spawnCwd="/r/api" persistKey="ctx1" />, { wrapper: Wrap });

    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(2));
    expect(screen.getAllByTestId("term").map((t) => t.textContent)).toEqual(["/r/api", "/r/web"]);
  });

  it("seeds one terminal when the context has no saved layout", async () => {
    render(<TerminalPanel spawnCwd="/r/api" persistKey="fresh" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
    expect(screen.getByTestId("term")).toHaveTextContent("/r/api");
  });

  it("saves tabs as they are opened and closed", async () => {
    render(<TerminalPanel spawnCwd="/r/api" persistKey="ctx2" />, { wrapper: Wrap });
    await waitFor(() => expect(loadTerminals("ctx2")).toHaveLength(1));

    fireEvent.click(screen.getByRole("button", { name: "new terminal" }));
    await waitFor(() => expect(loadTerminals("ctx2").map((t) => t.cwd)).toEqual(["/r/api", "/r/api"]));

    fireEvent.click(screen.getAllByRole("button", { name: /close terminal/ })[0]);
    await waitFor(() => expect(loadTerminals("ctx2")).toHaveLength(1));
  });

  it("persists nothing without a persistKey", async () => {
    render(<TerminalPanel spawnCwd="/r/api" />, { wrapper: Wrap });
    await waitFor(() => expect(screen.getAllByTestId("term")).toHaveLength(1));
    expect(localStorage.length).toBe(0);
  });
});
