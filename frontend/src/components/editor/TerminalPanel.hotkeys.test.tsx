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

  it("⌘T then I opens the insert-command menu (focus + showPicker)", async () => {
    // jsdom has no showPicker(); define it so the handler can call it, then clean up.
    const showPicker = vi.fn();
    (HTMLSelectElement.prototype as unknown as { showPicker: () => void }).showPicker = showPicker;
    try {
      await renderPanel();
      fireEvent.keyDown(window, { key: "t", metaKey: true });
      fireEvent.keyDown(window, { key: "i" });
      const select = screen.getByLabelText("Insert terminal command");
      expect(document.activeElement).toBe(select);
      expect(showPicker).toHaveBeenCalledTimes(1);
    } finally {
      delete (HTMLSelectElement.prototype as unknown as { showPicker?: () => void }).showPicker;
    }
  });
});
