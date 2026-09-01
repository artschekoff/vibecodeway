import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ResizeHandle } from "./ResizeHandle";

describe("ResizeHandle", () => {
  it("emits incremental x deltas for a vertical handle while dragging", () => {
    const onResize = vi.fn();
    render(<ResizeHandle orientation="vertical" onResize={onResize} ariaLabel="resize sidebar" />);
    const handle = screen.getByRole("separator", { name: "resize sidebar" });

    fireEvent.mouseDown(handle, { clientX: 100 });
    fireEvent.mouseMove(window, { clientX: 80 });
    expect(onResize).toHaveBeenLastCalledWith(-20);
    fireEvent.mouseMove(window, { clientX: 90 });
    expect(onResize).toHaveBeenLastCalledWith(10);
  });

  it("emits y deltas for a horizontal handle and stops after mouseup", () => {
    const onResize = vi.fn();
    render(<ResizeHandle orientation="horizontal" onResize={onResize} ariaLabel="resize terminal" />);
    const handle = screen.getByRole("separator", { name: "resize terminal" });

    fireEvent.mouseDown(handle, { clientY: 200 });
    fireEvent.mouseMove(window, { clientY: 150 });
    expect(onResize).toHaveBeenLastCalledWith(-50);

    fireEvent.mouseUp(window);
    onResize.mockClear();
    fireEvent.mouseMove(window, { clientY: 100 });
    expect(onResize).not.toHaveBeenCalled();
  });
});

describe("ResizeHandle grip", () => {
  it("renders a visible grip so the handle can be found and grabbed", () => {
    render(<ResizeHandle orientation="horizontal" onResize={() => {}} ariaLabel="resize terminal" />);
    const handle = screen.getByRole("separator", { name: "resize terminal" });

    // A hairline transparent strip is unhittable once a pane is dragged to the
    // edge of the layout — the grip is what makes the handle a target.
    const grip = handle.querySelector("[data-testid='resize-grip']");
    expect(grip).not.toBeNull();
    expect(handle).toHaveStyle({ cursor: "row-resize" });
  });

  it("still reports drag deltas with the grip present", () => {
    const onResize = vi.fn();
    render(<ResizeHandle orientation="horizontal" onResize={onResize} ariaLabel="resize terminal" />);
    const handle = screen.getByRole("separator", { name: "resize terminal" });

    fireEvent.mouseDown(handle, { clientY: 300 });
    fireEvent.mouseMove(window, { clientY: 260 });
    expect(onResize).toHaveBeenLastCalledWith(-40);
    fireEvent.mouseUp(window);
  });
});
