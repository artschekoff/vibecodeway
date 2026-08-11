import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { HotkeyProvider } from "../hotkeys/HotkeyProvider";
import { EditorFocusProvider, useEditorFocus, type Zone } from "./EditorFocusContext";

function ZoneStub() {
  const { active, register } = useEditorFocus();
  const zones: Zone[] = ["editor", "terminal", "sidebar"];
  return (
    <>
      {zones.map((z) => (
        <div key={z} data-testid={z} tabIndex={-1}
          ref={(el) => register(z, el)}
          className={active === z ? "zone--active" : ""} />
      ))}
    </>
  );
}

function renderStub() {
  return render(
    <HotkeyProvider platform="darwin" keybindings={[]}>
      <EditorFocusProvider>
        <ZoneStub />
      </EditorFocusProvider>
    </HotkeyProvider>
  );
}

describe("EditorFocusContext", () => {
  it("⌘E then j moves focus editor -> terminal", () => {
    renderStub();
    expect(screen.getByTestId("editor")).toHaveClass("zone--active");
    fireEvent.keyDown(window, { key: "e", metaKey: true });
    fireEvent.keyDown(window, { key: "j" });
    expect(screen.getByTestId("terminal")).toHaveClass("zone--active");
    expect(screen.getByTestId("editor")).not.toHaveClass("zone--active");
  });

  it("⌘E then l moves focus editor -> sidebar", () => {
    renderStub();
    fireEvent.keyDown(window, { key: "e", metaKey: true });
    fireEvent.keyDown(window, { key: "l" });
    expect(screen.getByTestId("sidebar")).toHaveClass("zone--active");
  });
});
