import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TabContextMenu } from "./TabContextMenu";

function setup(isPinned = false) {
  const onClose = vi.fn(), onCloseOthers = vi.fn(), onTogglePin = vi.fn(), onDismiss = vi.fn();
  render(<TabContextMenu x={10} y={20} isPinned={isPinned} onClose={onClose} onCloseOthers={onCloseOthers} onTogglePin={onTogglePin} onDismiss={onDismiss} />);
  return { onClose, onCloseOthers, onTogglePin, onDismiss };
}

describe("TabContextMenu", () => {
  it("Close fires onClose then onDismiss", () => {
    const { onClose, onDismiss } = setup();
    fireEvent.click(screen.getByRole("menuitem", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
    expect(onDismiss).toHaveBeenCalled();
  });

  it("Close all but this fires onCloseOthers", () => {
    const { onCloseOthers } = setup();
    fireEvent.click(screen.getByRole("menuitem", { name: "Close all but this" }));
    expect(onCloseOthers).toHaveBeenCalled();
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

  it("clamps position so the menu stays inside the viewport", () => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      width: 140, height: 84, top: 0, left: 0, right: 0, bottom: 0, x: 0, y: 0, toJSON: () => {},
    } as DOMRect);
    Object.defineProperty(window, "innerWidth", { value: 200, configurable: true });
    Object.defineProperty(window, "innerHeight", { value: 100, configurable: true });

    const noop = vi.fn();
    render(<TabContextMenu x={190} y={95} isPinned={false} onClose={noop} onCloseOthers={noop} onTogglePin={noop} onDismiss={noop} />);

    const menu = screen.getByRole("menu");
    // left = min(190, 200-140-8) = 52 ; top = min(95, 100-84-8) = 8
    expect(menu.style.left).toBe("52px");
    expect(menu.style.top).toBe("8px");

    vi.restoreAllMocks();
  });
});
