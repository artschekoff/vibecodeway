import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const disposeTerm = vi.fn();
const disposeInput = vi.fn();

vi.mock("@xterm/xterm", () => ({
  Terminal: vi.fn().mockImplementation(function () {
    return {
      loadAddon: vi.fn(), open: vi.fn(), write: vi.fn(),
      onData: () => ({ dispose: disposeInput }),
      dispose: disposeTerm, cols: 80, rows: 24,
    };
  }),
}));
vi.mock("@xterm/addon-fit", () => ({ FitAddon: vi.fn().mockImplementation(function () { return { fit: vi.fn() }; }) }));

const disconnectSpy = vi.fn();
class ResizeObserverMock {
  observe = vi.fn(); unobserve = vi.fn(); disconnect = disconnectSpy;
}
globalThis.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;
Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, value: 800 });
Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, value: 600 });

const { TerminalStart, TerminalWrite, TerminalClose, TerminalResize } = vi.hoisted(() => ({
  TerminalStart: vi.fn(), TerminalWrite: vi.fn(), TerminalClose: vi.fn(), TerminalResize: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ TerminalStart, TerminalWrite, TerminalClose, TerminalResize }));

// Capture the term:exit subscription so the test can fire a real shell exit.
const exitHandlers = new Map<string, () => void>();
const offData = vi.fn();
const offExit = vi.fn();
const EventsOff = vi.fn();
vi.mock("../../../wailsjs/runtime/runtime", () => ({
  EventsOn: (event: string, cb: () => void) => {
    if (event.startsWith("term:exit:")) { exitHandlers.set(event, cb); return offExit; }
    return offData;
  },
  EventsOff: (...args: string[]) => EventsOff(...args),
}));

import { Terminal } from "./Terminal";

beforeEach(() => {
  vi.clearAllMocks();
  exitHandlers.clear();
  TerminalStart.mockResolvedValue("sess-1");
});

// Closing a context or editor tab unmounts this component. If the cleanup let
// go of the PTY, the shell would keep running with nobody reading it.
describe("Terminal disposal", () => {
  it("kills the PTY and tears everything down on unmount", async () => {
    const { unmount } = render(<Terminal cwd="/repo/x" />);
    await waitFor(() => expect(TerminalStart).toHaveBeenCalledWith("/repo/x"));

    unmount();

    expect(TerminalClose).toHaveBeenCalledWith("sess-1");
    expect(disconnectSpy).toHaveBeenCalled();
    expect(disposeInput).toHaveBeenCalled();
    expect(disposeTerm).toHaveBeenCalled();
    expect(offData).toHaveBeenCalled();
    expect(offExit).toHaveBeenCalled();
    expect(EventsOff).toHaveBeenCalledWith("term:data:sess-1");
    expect(EventsOff).toHaveBeenCalledWith("term:exit:sess-1");
  });

  it("closes a session that arrives after unmount, instead of leaking it", async () => {
    // TerminalStart is in flight when the tab is closed: the resolved id must
    // still be killed, or the shell outlives the component with no owner.
    let resolveStart: (id: string) => void = () => {};
    TerminalStart.mockReturnValue(new Promise<string>((r) => { resolveStart = r; }));

    const { unmount } = render(<Terminal cwd="/repo/x" />);
    unmount();
    resolveStart("late-sess");

    await waitFor(() => expect(TerminalClose).toHaveBeenCalledWith("late-sess"));
  });

  it("reports the shell exiting so the owner can close the tab", async () => {
    const onExit = vi.fn();
    render(<Terminal cwd="/repo/x" onExit={onExit} />);
    await waitFor(() => expect(exitHandlers.has("term:exit:sess-1")).toBe(true));

    exitHandlers.get("term:exit:sess-1")!();
    expect(onExit).toHaveBeenCalled();
  });
});
