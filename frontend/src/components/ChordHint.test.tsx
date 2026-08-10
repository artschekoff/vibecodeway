import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { HotkeyProvider } from "../hotkeys/HotkeyProvider";
import { ChordHint } from "./ChordHint";
import type { Keybinding } from "../types";

function kb(over: Partial<Keybinding>): Keybinding {
  return {
    id: "", command: "", label: "", description: "",
    modifierKeys: "", keyCode: "", sequence: "",
    object: "", action: "", scope: "global",
    defaultModifierKeys: "", defaultKeyCode: "", defaultSequence: "",
    updatedAt: "", ...over,
  };
}

describe("ChordHint", () => {
  it("renders the live default chord for a command", () => {
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <button>Open Editor<ChordHint id="editor.open" /></button>
      </HotkeyProvider>
    );
    expect(screen.getByRole("button")).toHaveTextContent("(⌘E)");
  });

  it("reflects a rebind", () => {
    render(
      <HotkeyProvider platform="darwin" keybindings={[kb({ command: "editor.open", modifierKeys: "cmd", keyCode: "j" })]}>
        <button>Open Editor<ChordHint id="editor.open" /></button>
      </HotkeyProvider>
    );
    expect(screen.getByRole("button")).toHaveTextContent("(⌘J)");
  });
});
