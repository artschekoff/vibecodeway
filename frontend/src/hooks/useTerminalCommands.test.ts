import { renderHook, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { ListTerminalCommands } = vi.hoisted(() => ({ ListTerminalCommands: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => ({ ListTerminalCommands }));

import { useTerminalCommands } from "./useTerminalCommands";

beforeEach(() => {
  vi.clearAllMocks();
  ListTerminalCommands.mockResolvedValue([
    { id: "c1", title: "Inject", command: "/load-context <context-name>", createdAt: "", updatedAt: "" },
  ]);
});

describe("useTerminalCommands", () => {
  it("loads the command list", async () => {
    const { result } = renderHook(() => useTerminalCommands());
    await waitFor(() => expect(result.current.commands).toHaveLength(1));
    expect(result.current.commands[0].title).toBe("Inject");
  });

  it("reloads when a 'terminal-commands-changed' event fires (e.g. edited in Settings)", async () => {
    const { result } = renderHook(() => useTerminalCommands());
    await waitFor(() => expect(result.current.commands[0].sendEnter).toBeFalsy());
    // Simulate the snippet being edited/saved elsewhere: now it has sendEnter.
    ListTerminalCommands.mockResolvedValue([
      { id: "c1", title: "Inject", command: "x", sendEnter: true, createdAt: "", updatedAt: "" },
    ]);
    window.dispatchEvent(new CustomEvent("terminal-commands-changed"));
    await waitFor(() => expect(result.current.commands[0].sendEnter).toBe(true));
  });
});
