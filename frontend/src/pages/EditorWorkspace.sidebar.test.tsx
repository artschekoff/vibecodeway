import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useState } from "react";
import { HotkeyProvider, useHotkey, useHotkeyScope } from "../hotkeys/HotkeyProvider";

// Mirrors EditorWorkspace's sidebar wiring: two direct handlers under scope
// "editor" bound to the ⌘B leader (⌘B F = files, ⌘B G = git). A competing
// terminal.next (scope "terminal") on ⌘] proves the chords no longer collide.
function SidebarHarness({ terminalFocused }: { terminalFocused: boolean }) {
  const [tab, setTab] = useState<"files" | "git">("git");
  const [terminalHits, setTerminalHits] = useState(0);

  useHotkeyScope("editor", true);
  useHotkey("sidebar.files", () => setTab("files"), { scope: "editor" });
  useHotkey("sidebar.git", () => setTab("git"), { scope: "editor" });

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
  it("⌘B F shows FILES, ⌘B G shows GIT when editor scope is active", () => {
    renderHarness();
    expect(screen.getByTestId("tab")).toHaveTextContent("git");
    fireEvent.keyDown(window, { key: "b", metaKey: true }); // leader
    fireEvent.keyDown(window, { key: "f" });                // -> files
    expect(screen.getByTestId("tab")).toHaveTextContent("files");
    fireEvent.keyDown(window, { key: "b", metaKey: true });
    fireEvent.keyDown(window, { key: "g" });                // -> git
    expect(screen.getByTestId("tab")).toHaveTextContent("git");
  });

  it("⌘] routes to terminal.next when the terminal zone is focused (no sidebar collision)", () => {
    renderHarness(true);
    fireEvent.keyDown(window, { key: "]", metaKey: true });
    expect(screen.getByTestId("terminal-hits")).toHaveTextContent("1");
    expect(screen.getByTestId("tab")).toHaveTextContent("git"); // sidebar untouched
  });
});
