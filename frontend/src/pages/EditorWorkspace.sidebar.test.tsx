import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useState } from "react";
import { HotkeyProvider, useHotkey, useHotkeyScope } from "../hotkeys/HotkeyProvider";

// Mirrors EditorWorkspace's sidebar wiring: the two handlers register under
// scope "editor". A competing terminal.next (scope "terminal") lets us prove the
// SAME chord (⌘⇧]) routes by scope priority — terminal wins when its zone is focused.
function SidebarHarness({ terminalFocused }: { terminalFocused: boolean }) {
  const [tab, setTab] = useState<"files" | "git">("files");
  const [terminalHits, setTerminalHits] = useState(0);
  const toggle = () => setTab((t) => (t === "files" ? "git" : "files"));

  useHotkeyScope("editor", true);
  useHotkey("sidebar.next", toggle, { scope: "editor" });
  useHotkey("sidebar.prev", toggle, { scope: "editor" });

  // Stand-in for the Phase-4 terminal.next handler.
  useHotkeyScope("terminal", terminalFocused);
  useHotkey("terminal.next", () => setTerminalHits((n) => n + 1), { scope: "terminal" });

  return (
    <div>
      <span data-testid="tab">{tab}</span>
      <span data-testid="terminal-hits">{terminalHits}</span>
    </div>
  );
}

function renderHarness(terminalFocused = false) {
  return render(
    <HotkeyProvider platform="darwin" keybindings={[]}>
      <SidebarHarness terminalFocused={terminalFocused} />
    </HotkeyProvider>
  );
}

describe("EditorWorkspace sidebar switch", () => {
  it("⌘⇧] toggles FILES -> GIT when editor scope is active", () => {
    renderHarness();
    expect(screen.getByTestId("tab")).toHaveTextContent("files");
    fireEvent.keyDown(window, { key: "]", metaKey: true, shiftKey: true });
    expect(screen.getByTestId("tab")).toHaveTextContent("git");
  });

  it("does NOT switch the sidebar when the terminal zone is focused (chord routes to terminal.next)", () => {
    renderHarness(true);
    fireEvent.keyDown(window, { key: "]", metaKey: true, shiftKey: true });
    expect(screen.getByTestId("tab")).toHaveTextContent("files");
    expect(screen.getByTestId("terminal-hits")).toHaveTextContent("1");
  });
});
