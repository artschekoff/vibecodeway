import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TabContextMenu } from "./TabContextMenu";

function setup(isPinned = false) {
  const onClose = vi.fn(), onCloseAll = vi.fn(), onTogglePin = vi.fn(), onDismiss = vi.fn();
  render(<TabContextMenu x={10} y={20} isPinned={isPinned} onClose={onClose} onCloseAll={onCloseAll} onTogglePin={onTogglePin} onDismiss={onDismiss} />);
  return { onClose, onCloseAll, onTogglePin, onDismiss };
}

describe("TabContextMenu", () => {
  it("Close fires onClose then onDismiss", () => {
    const { onClose, onDismiss } = setup();
    fireEvent.click(screen.getByRole("menuitem", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
    expect(onDismiss).toHaveBeenCalled();
  });

  it("Close all fires onCloseAll", () => {
    const { onCloseAll } = setup();
    fireEvent.click(screen.getByRole("menuitem", { name: "Close all" }));
    expect(onCloseAll).toHaveBeenCalled();
  });

  it("shows Pin when unpinned and Unpin when pinned", () => {
    const { onTogglePin } = setup(false);
    fireEvent.click(screen.getByRole("menuitem", { name: "Pin" }));
    expect(onTogglePin).toHaveBeenCalled();
  });

  it("shows Unpin label when pinned", () => {
    setup(true);
    expect(screen.getByRole("menuitem", { name: "Unpin" })).toBeInTheDocument();
  });

  it("clicking the backdrop dismisses", () => {
    const { onDismiss } = setup();
    fireEvent.click(screen.getByTestId("tab-context-menu-backdrop"));
    expect(onDismiss).toHaveBeenCalled();
  });
});
