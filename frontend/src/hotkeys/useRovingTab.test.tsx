import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useRef } from "react";
import { useRovingTab } from "./useRovingTab";

// jsdom returns zeroed rects; stub each button so ttb-rtl order is deterministic.
function stubRect(el: HTMLElement, top: number, right: number) {
  el.getBoundingClientRect = () =>
    ({ top, right, bottom: top + 20, left: right - 40, width: 40, height: 20, x: right - 40, y: top, toJSON() {} }) as DOMRect;
}

function Zone() {
  const ref = useRef<HTMLDivElement>(null);
  useRovingTab(ref, { order: "ttb-rtl" });
  return (
    <div ref={ref}>
      <button data-testid="a">A</button>
      <button data-testid="b">B</button>
      <button data-testid="c">C</button>
    </div>
  );
}

describe("useRovingTab (ttb-rtl)", () => {
  beforeEach(() => {
    render(<Zone />);
    // Row 1 (top 0): B is right of A → B before A. Row 2 (top 50): C.
    stubRect(screen.getByTestId("a"), 0, 100);
    stubRect(screen.getByTestId("b"), 0, 200);
    stubRect(screen.getByTestId("c"), 50, 100);
  });

  it("Tab moves top→bottom, right→left and wraps", () => {
    const a = screen.getByTestId("a");
    a.focus();
    // order = [B, A, C]; from A → C → B → A
    fireEvent.keyDown(a, { key: "Tab" });
    expect(screen.getByTestId("c")).toHaveFocus();
    fireEvent.keyDown(screen.getByTestId("c"), { key: "Tab" });
    expect(screen.getByTestId("b")).toHaveFocus();
    fireEvent.keyDown(screen.getByTestId("b"), { key: "Tab" });
    expect(screen.getByTestId("a")).toHaveFocus();
  });

  it("Shift+Tab moves in reverse", () => {
    const b = screen.getByTestId("b");
    b.focus();
    // order = [B, A, C]; from B (index 0) back → C (wrap)
    fireEvent.keyDown(b, { key: "Tab", shiftKey: true });
    expect(screen.getByTestId("c")).toHaveFocus();
  });
});
