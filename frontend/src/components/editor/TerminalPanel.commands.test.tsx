import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock Terminal: reports a fake session id, prints cwd.
vi.mock("./Terminal", () => ({
  Terminal: ({ cwd, onSession }: { cwd: string; onSession?: (id: string) => void }) => {
    onSession?.("sess-1");
    return <div data-testid="term">{cwd}</div>;
  },
}));

const { TerminalWrite, ListTerminalCommands } = vi.hoisted(() => ({
  TerminalWrite: vi.fn(),
  ListTerminalCommands: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ TerminalWrite, ListTerminalCommands }));

import { TerminalPanel } from "./TerminalPanel";

beforeEach(() => {
  vi.clearAllMocks();
  TerminalWrite.mockResolvedValue(undefined);
  ListTerminalCommands.mockResolvedValue([
    { id: "c1", title: "Inject", command: "/load-context <context-name>", sendEnter: false, createdAt: "", updatedAt: "" },
    { id: "c2", title: "Run", command: "npm test", sendEnter: true, createdAt: "", updatedAt: "" },
  ]);
});

describe("TerminalPanel command selector", () => {
  it("substitutes <context-name> and types the command without Enter", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="tell-us-more-fin" />);
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    const select = await screen.findByLabelText("Insert terminal command");
    fireEvent.change(select, { target: { value: "c1" } });
    await waitFor(() =>
      expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "/load-context tell-us-more-fin")
    );
  });

  it("appends Enter (\\r) when the command has sendEnter", async () => {
    render(<TerminalPanel spawnCwd="/r/api" contextName="ctx" />);
    await waitFor(() => expect(screen.getByTestId("term")).toBeInTheDocument());
    const select = await screen.findByLabelText("Insert terminal command");
    fireEvent.change(select, { target: { value: "c2" } });
    await waitFor(() => expect(TerminalWrite).toHaveBeenCalledWith("sess-1", "npm test\r"));
  });
});
