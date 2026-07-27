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
