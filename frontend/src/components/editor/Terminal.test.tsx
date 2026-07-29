import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const onDataHandlers: Array<(d: string) => void> = [];
const writeSpy = vi.fn();
const fitSpy = vi.fn();

vi.mock("@xterm/xterm", () => ({
  Terminal: vi.fn().mockImplementation(function () {
    return {
      loadAddon: vi.fn(),
      open: vi.fn(),
      write: writeSpy,
      onData: (cb: (d: string) => void) => { onDataHandlers.push(cb); return { dispose: vi.fn() }; },
      dispose: vi.fn(),
      cols: 80,
      rows: 24,
    };
  }),
}));
vi.mock("@xterm/addon-fit", () => ({ FitAddon: vi.fn().mockImplementation(function () { return { fit: fitSpy }; }) }));

// jsdom has no ResizeObserver; capture the callback so we can simulate a resize.
let roCallback: ResizeObserverCallback | null = null;
const observeSpy = vi.fn();
const disconnectSpy = vi.fn();
class ResizeObserverMock {
  constructor(cb: ResizeObserverCallback) { roCallback = cb; }
  observe = observeSpy;
  unobserve = vi.fn();
  disconnect = disconnectSpy;
}
globalThis.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;

// jsdom reports offsetWidth/offsetHeight as 0 for every element. Terminal skips
// fit/resize when its host has no layout box (hidden tab); give elements a real
// size here so the "visible terminal" path (fit + PTY resize) is exercised.
Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, value: 800 });
Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, value: 600 });

const { TerminalStart, TerminalWrite, TerminalClose, TerminalResize } = vi.hoisted(() => ({
  TerminalStart: vi.fn(), TerminalWrite: vi.fn(), TerminalClose: vi.fn(), TerminalResize: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ TerminalStart, TerminalWrite, TerminalClose, TerminalResize }));
vi.mock("../../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => vi.fn()), EventsOff: vi.fn() }));

import { Terminal } from "./Terminal";

beforeEach(() => {
  onDataHandlers.length = 0;
  vi.clearAllMocks();
  TerminalStart.mockResolvedValue("sess-1");
});

describe("Terminal", () => {
  it("starts a PTY session at cwd and forwards keystrokes", async () => {
    render(<Terminal cwd="/repo/x" />);
    await waitFor(() => expect(TerminalStart).toHaveBeenCalledWith("/repo/x"));
    onDataHandlers[0]?.("ls\n");
    expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "ls\n");
  });

  it("refits and resizes the PTY when its container resizes", async () => {
    render(<Terminal cwd="/repo/x" />);
    await waitFor(() => expect(TerminalStart).toHaveBeenCalled());
    // Session established → initial resize sent.
    await waitFor(() => expect(TerminalResize).toHaveBeenCalled());
    expect(observeSpy).toHaveBeenCalled();

    fitSpy.mockClear();
    TerminalResize.mockClear();

    // Simulate a pane-drag resize (no window 'resize' event fires for that).
    roCallback?.([], {} as ResizeObserver);

    expect(fitSpy).toHaveBeenCalled();
    expect(TerminalResize).toHaveBeenCalledWith("sess-1", 80, 24);
  });
});
