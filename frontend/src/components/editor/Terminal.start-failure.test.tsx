import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const writeSpy = vi.fn();
const fitSpy = vi.fn();

vi.mock("@xterm/xterm", () => ({
  Terminal: vi.fn().mockImplementation(function () {
    return {
      loadAddon: vi.fn(),
      open: vi.fn(),
      write: writeSpy,
      onData: vi.fn(() => ({ dispose: vi.fn() })),
      dispose: vi.fn(),
      cols: 80,
      rows: 24,
    };
  }),
}));
vi.mock("@xterm/addon-fit", () => ({ FitAddon: vi.fn().mockImplementation(function () { return { fit: fitSpy }; }) }));

class ResizeObserverMock {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
globalThis.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;

Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, value: 800 });
Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, value: 600 });

const { TerminalStart, TerminalWrite, TerminalClose, TerminalResize } = vi.hoisted(() => ({
  TerminalStart: vi.fn(), TerminalWrite: vi.fn(), TerminalClose: vi.fn(), TerminalResize: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ TerminalStart, TerminalWrite, TerminalClose, TerminalResize }));
vi.mock("../../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => vi.fn()), EventsOff: vi.fn() }));

import { Terminal } from "./Terminal";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("Terminal — PTY start failure", () => {
  it("writes a message into the buffer and never retires the tab when the folder is gone", async () => {
    TerminalStart.mockRejectedValue(new Error("chdir /r/deleted: no such file or directory"));
    const onSession = vi.fn();

    render(<Terminal cwd="/r/deleted" onSession={onSession} />);

    await waitFor(() =>
      expect(writeSpy).toHaveBeenCalledWith(
        "\r\n[cannot open a terminal in /r/deleted: chdir /r/deleted: no such file or directory]\r\n",
      ));
    // No session ever came up, so the panel must not be told this tab exited
    // (there is no PTY session to tear down) or that a session started — the
    // tab stays visible with the message on screen instead of being retired.
    expect(onSession).not.toHaveBeenCalled();
    expect(TerminalClose).not.toHaveBeenCalled();
  });
});
