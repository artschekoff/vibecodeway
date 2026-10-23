import { describe, it, expect, beforeEach, afterEach } from "vitest";

// jsdom reports zero-size rects, and zero-size zones get no badge.
const origRect = Element.prototype.getBoundingClientRect;
beforeEach(() => {
  Element.prototype.getBoundingClientRect = function () {
    return { width: 100, height: 100, left: 5, top: 5, right: 105, bottom: 105, x: 5, y: 5, toJSON: () => ({}) } as DOMRect;
  };
});
afterEach(() => { Element.prototype.getBoundingClientRect = origRect; });
import { render, screen, fireEvent } from "@testing-library/react";
import { EditorFocusProvider, useEditorFocus } from "./EditorFocusContext";
import { HotkeyProvider } from "../hotkeys/HotkeyProvider";

function Zones() {
  const { active, register } = useEditorFocus();
  return (
    <div>
      <div data-testid="active">{active}</div>
      <div ref={(el) => register("editor", el)} tabIndex={-1}>E</div>
      <div ref={(el) => register("terminal", el)} tabIndex={-1}>T</div>
      <div ref={(el) => register("sidebar", el)} tabIndex={-1}>S</div>
    </div>
  );
}

const renderZones = () =>
  render(
    <HotkeyProvider platform="darwin" keybindings={[]}>
      <EditorFocusProvider><Zones /></EditorFocusProvider>
    </HotkeyProvider>
  );

describe("⌘-hold zone navigation", () => {
  it("shows numbered badges while ⌘ is held and hides on release", () => {
    renderZones();
    fireEvent.keyDown(window, { key: "Meta", metaKey: true });
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    fireEvent.keyUp(window, { key: "Meta" });
    expect(screen.queryByText("1")).not.toBeInTheDocument();
  });

  it("⌘2 focuses the terminal zone", () => {
    renderZones();
    fireEvent.keyDown(window, { key: "Meta", metaKey: true });
    fireEvent.keyDown(window, { key: "2", metaKey: true });
    expect(screen.getByTestId("active")).toHaveTextContent("terminal");
    expect(screen.queryByText("2")).not.toBeInTheDocument(); // badges gone after jump
  });
});
