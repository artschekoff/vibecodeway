import { useRef } from "react";

type Orientation = "vertical" | "horizontal";

type Props = {
  // "vertical" = a vertical bar the user drags left/right to resize a width;
  // "horizontal" = a horizontal bar dragged up/down to resize a height.
  orientation: Orientation;
  // Incremental px delta since the previous move event (+x right / +y down).
  onResize: (delta: number) => void;
  ariaLabel: string;
};

export function ResizeHandle({ orientation, onResize, ariaLabel }: Props) {
  const last = useRef(0);

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    last.current = orientation === "vertical" ? e.clientX : e.clientY;
    const prevUserSelect = document.body.style.userSelect;
    document.body.style.userSelect = "none"; // avoid text selection while dragging

    const onMove = (ev: MouseEvent) => {
      const pos = orientation === "vertical" ? ev.clientX : ev.clientY;
      const delta = pos - last.current;
      last.current = pos;
      if (delta !== 0) onResize(delta);
    };
    const onUp = () => {
      document.body.style.userSelect = prevUserSelect;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  return (
    <div
      role="separator"
      aria-label={ariaLabel}
      aria-orientation={orientation}
      onMouseDown={handleMouseDown}
      style={
        orientation === "vertical"
          ? { width: 5, cursor: "col-resize", flexShrink: 0, background: "transparent" }
          : { height: 5, cursor: "row-resize", flexShrink: 0, background: "transparent" }
      }
    />
  );
}
