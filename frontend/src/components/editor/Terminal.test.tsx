import { render, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const onDataHandlers: Array<(d: string) => void> = [];
const writeSpy = vi.fn();

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
vi.mock("@xterm/addon-fit", () => ({ FitAddon: vi.fn().mockImplementation(function () { return { fit: vi.fn() }; }) }));

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
});
