import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { useEffect } from "react";

vi.mock("./Terminal", () => ({
  Terminal: ({ cwd, onSession }: { cwd: string; onSession?: (id: string) => void }) => {
    // The real Terminal calls onSession once its PTY is up; mimic that so the
    // panel's resume-on-spawn path is exercised.
    useEffect(() => { onSession?.(`sess-for-${cwd}`); }, [cwd, onSession]);
    return <div data-testid="term">{cwd}</div>;
  },
}));

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

describe("TerminalPanel — agent resume", () => {
  it("types the stored resume command into a restored agent tab", async () => {
    saveTerminals("ctx3", [{ cwd: "/r/api", openedAt: 1, resume: "claude --resume s1" }]);

    render(<TerminalPanel spawnCwd="/r/api" persistKey="ctx3" />, { wrapper: Wrap });

    await waitFor(() =>
      expect(H.TerminalWrite).toHaveBeenCalledWith("sess-for-/r/api", "claude --resume s1\r"));
    // Once only — a re-render must not re-run the agent.
    expect(H.TerminalWrite).toHaveBeenCalledTimes(1);
  });

  it("leaves a restored plain shell alone", async () => {
    saveTerminals("ctx4", [{ cwd: "/r/api", openedAt: 1 }]);

    render(<TerminalPanel spawnCwd="/r/api" persistKey="ctx4" />, { wrapper: Wrap });

    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    expect(H.TerminalWrite).not.toHaveBeenCalled();
  });

  it("records the resume command the backend reports for an open tab", async () => {
    H.AgentResumeCommand.mockResolvedValue("claude --resume s9");

    render(<TerminalPanel spawnCwd="/r/api" persistKey="ctx5" />, { wrapper: Wrap });

    await waitFor(() =>
      expect(loadTerminals("ctx5")).toEqual([
        expect.objectContaining({ cwd: "/r/api", resume: "claude --resume s9" }),
      ]));
    // A freshly opened tab is not itself resumed — only future restores use this.
    expect(H.TerminalWrite).not.toHaveBeenCalled();
  });
});
