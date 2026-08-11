import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { HotkeyProvider, useHotkey } from "../hotkeys/HotkeyProvider";
import { HotkeyHelpModal } from "./HotkeyHelpModal";

describe("HotkeyHelpModal", () => {
  it("opens on help.open and filters by chord or by action name", async () => {
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <HotkeyHelpModal />
      </HotkeyProvider>
    );

    // Not rendered until opened.
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    fireEvent.keyDown(window, { key: "h", metaKey: true });
    expect(await screen.findByRole("dialog")).toBeInTheDocument();

    const filter = screen.getByPlaceholderText(/filter/i);

    fireEvent.change(filter, { target: { value: "⌘T" } });
    expect(screen.getAllByText(/Terminal/).length).toBeGreaterThan(0);

    fireEvent.change(filter, { target: { value: "reload" } });
    expect(screen.getByText(/Reload files/)).toBeInTheDocument();

    // camelCase label fix: "context.copyName" -> "Copy Name" (not "CopyName")
    fireEvent.change(filter, { target: { value: "copy name" } });
    expect(screen.getByText(/Copy Name/)).toBeInTheDocument();
    expect(screen.queryByText(/CopyName/)).not.toBeInTheDocument();

    // object "pr" -> "PR"
    fireEvent.change(filter, { target: { value: "" } });
    expect(screen.getAllByText(/^PR$/).length).toBeGreaterThan(0);

    // Escape closes.
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("runs the highlighted command on Enter and closes — even when its scope is inactive", async () => {
    const spy = vi.fn();
    function Harness() {
      // Registered under the "terminal" scope but that scope is NOT active
      // here (no terminal focus). Keyboard chords wouldn't fire it, but
      // picking it from the palette must.
      useHotkey("terminal.new", spy, { scope: "terminal" });
      return <HotkeyHelpModal />;
    }
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <Harness />
      </HotkeyProvider>
    );

    fireEvent.keyDown(window, { key: "h", metaKey: true });
    await screen.findByRole("dialog");

    const filter = screen.getByPlaceholderText(/filter/i);
    fireEvent.change(filter, { target: { value: "new terminal" } });

    fireEvent.keyDown(window, { key: "Enter" });

    expect(spy).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
