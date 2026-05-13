import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import KeybindingRow from "./KeybindingRow";
import type { Keybinding } from "../types";

function makeBinding(overrides: Partial<Keybinding> = {}): Keybinding {
  return {
    id: "kb-quick-attach",
    command: "quick_attach",
    label: "Quick Attach",
    description: "Select text and open context picker globally.",
    modifierKeys: "cmd,shift",
    keyCode: "c",
    updatedAt: "",
    ...overrides,
  };
}

describe("KeybindingRow", () => {
  it("renders the label", () => {
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    expect(screen.getByText("Quick Attach")).toBeInTheDocument();
  });

  it("renders the description", () => {
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    expect(screen.getByText(/Select text and open context picker/i)).toBeInTheDocument();
  });

  it("renders the current combo as kbd elements", () => {
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    expect(screen.getByText("⌘")).toBeInTheDocument();
    expect(screen.getByText("⇧")).toBeInTheDocument();
    expect(screen.getByText("C")).toBeInTheDocument();
  });

  it("renders a Change button", () => {
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    expect(screen.getByRole("button", { name: /change/i })).toBeInTheDocument();
  });

  it("enters listening state on Change click", async () => {
    const user = userEvent.setup();
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /change/i }));
    expect(screen.getByText(/press a key combination/i)).toBeInTheDocument();
  });

  it("exits listening state on Escape", async () => {
    const user = userEvent.setup();
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /change/i }));
    await user.keyboard("{Escape}");
    expect(screen.queryByText(/press a key combination/i)).not.toBeInTheDocument();
  });

  it("captures a new combo and shows a Save button", async () => {
    const user = userEvent.setup();
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /change/i }));
    // Simulate Cmd+Shift+X (metaKey + shiftKey + "x")
    await user.keyboard("{Meta>}{Shift>}x{/Shift}{/Meta}");
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });

  it("calls onSave with new modifiers and key when Save is clicked", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(<KeybindingRow binding={makeBinding()} onSave={onSave} />);
    await user.click(screen.getByRole("button", { name: /change/i }));
    await user.keyboard("{Meta>}{Shift>}x{/Shift}{/Meta}");
    await user.click(screen.getByRole("button", { name: /save/i }));
    expect(onSave).toHaveBeenCalledWith("quick_attach", "cmd,shift", "x");
  });

  it("shows Cancel button alongside Save", async () => {
    const user = userEvent.setup();
    render(<KeybindingRow binding={makeBinding()} onSave={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: /change/i }));
    await user.keyboard("{Meta>}{Shift>}x{/Shift}{/Meta}");
    expect(screen.getByRole("button", { name: /cancel/i })).toBeInTheDocument();
  });

  it("discards pending combo on Cancel", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(<KeybindingRow binding={makeBinding()} onSave={onSave} />);
    await user.click(screen.getByRole("button", { name: /change/i }));
    await user.keyboard("{Meta>}{Shift>}x{/Shift}{/Meta}");
    await user.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /change/i })).toBeInTheDocument();
  });
});
