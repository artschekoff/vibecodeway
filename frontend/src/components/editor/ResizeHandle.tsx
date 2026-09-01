import { useRef, useState } from "react";

type Orientation = "vertical" | "horizontal";

type Props = {
  // "vertical" = a vertical bar the user drags left/right to resize a width;
  // "horizontal" = a horizontal bar dragged up/down to resize a height.
  orientation: Orientation;
  // Incremental px delta since the previous move event (+x right / +y down).
  onResize: (delta: number) => void;
  ariaLabel: string;
};

// The strip stays thin, but a visible grip sits in the middle of it. A fully
// transparent hairline is findable only by luck, and impossible to recover from
// once a pane has been dragged to the edge of the layout.
const THICKNESS = 9;

export function ResizeHandle({ orientation, onResize, ariaLabel }: Props) {
  const last = useRef(0);
  const [active, setActive] = useState(false);
  const [hover, setHover] = useState(false);
  const horizontal = orientation === "horizontal";

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    last.current = horizontal ? e.clientY : e.clientX;
    setActive(true);
    const prevUserSelect = document.body.style.userSelect;
    document.body.style.userSelect = "none"; // avoid text selection while dragging

    const onMove = (ev: MouseEvent) => {
      const pos = horizontal ? ev.clientY : ev.clientX;
      const delta = pos - last.current;
      last.current = pos;
      if (delta !== 0) onResize(delta);
    };
    const onUp = () => {
      document.body.style.userSelect = prevUserSelect;
      setActive(false);
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const lit = active || hover;

  return (
    <div
      role="separator"
      aria-label={ariaLabel}
      aria-orientation={orientation}
      onMouseDown={handleMouseDown}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      style={{
        ...(horizontal ? { height: THICKNESS } : { width: THICKNESS }),
        cursor: horizontal ? "row-resize" : "col-resize",
        flexShrink: 0,
        background: "transparent",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <div
        data-testid="resize-grip"
        style={{
          // A short pill along the drag axis, centred in the strip. Dim until
          // pointed at, so it reads as an affordance without drawing the eye.
          ...(horizontal ? { width: 44, height: 3 } : { width: 3, height: 44 }),
          borderRadius: 3,
          background: lit ? "var(--accent, #7aa2f7)" : "var(--border)",
          transition: "background 120ms ease",
        }}
      />
    </div>
  );
}
