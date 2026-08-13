import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { HotkeyProvider, useHotkey } from "../hotkeys/HotkeyProvider";
import { HotkeyHelpModal } from "./HotkeyHelpModal";

vi.mock("../../wailsjs/go/main/App", () => ({
  UpdateKeybinding: vi.fn().mockResolvedValue(undefined),
  ResetKeybinding: vi.fn().mockResolvedValue(undefined),
}));
import { UpdateKeybinding, ResetKeybinding } from "../../wailsjs/go/main/App";

beforeEach(() => vi.clearAllMocks());

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

  it("runs the highlighted command from the Execute button", async () => {
    const spy = vi.fn();
    function Harness() {
      useHotkey("terminal.new", spy, { scope: "global" });
      return <HotkeyHelpModal />;
    }
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <Harness />
      </HotkeyProvider>
    );
    fireEvent.keyDown(window, { key: "h", metaKey: true });
    await screen.findByRole("dialog");
    fireEvent.change(screen.getByPlaceholderText(/filter/i), { target: { value: "new terminal" } });

    fireEvent.click(screen.getByRole("button", { name: /execute/i }));

    expect(spy).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("captures a new key combination and saves it, then broadcasts a change", async () => {
    const changed = vi.fn();
    window.addEventListener("keybindings-changed", changed);
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <HotkeyHelpModal />
      </HotkeyProvider>
    );
    fireEvent.keyDown(window, { key: "h", metaKey: true });
    await screen.findByRole("dialog");
    fireEvent.change(screen.getByPlaceholderText(/filter/i), { target: { value: "reload files" } });

    // Enter capture mode via the pencil ("Change shortcut") button.
    fireEvent.click(screen.getByRole("button", { name: /change shortcut/i }));
    expect(screen.getByText(/press a combination/i)).toBeInTheDocument();

    // A base combo, then a trailing plain key (leader sequence) → auto-save.
    fireEvent.keyDown(window, { key: "j", metaKey: true, shiftKey: true });
    fireEvent.keyDown(window, { key: "k" });

    expect(UpdateKeybinding).toHaveBeenCalledWith("files.reload", "cmd,shift", "j", "k");
    await screen.findByText(/reload files/i); // still open, save resolved
    expect(changed).toHaveBeenCalled();
    window.removeEventListener("keybindings-changed", changed);
  });

  it("resets a binding from the reset button", async () => {
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <HotkeyHelpModal />
      </HotkeyProvider>
    );
    fireEvent.keyDown(window, { key: "h", metaKey: true });
    await screen.findByRole("dialog");
    fireEvent.change(screen.getByPlaceholderText(/filter/i), { target: { value: "reload files" } });

    fireEvent.click(screen.getByRole("button", { name: /reset to default/i }));
    expect(ResetKeybinding).toHaveBeenCalledWith("files.reload");
  });
});
