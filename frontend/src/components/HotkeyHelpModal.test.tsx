import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { HotkeyProvider } from "../hotkeys/HotkeyProvider";
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
});
