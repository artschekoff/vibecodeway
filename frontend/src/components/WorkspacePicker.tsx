import { useEffect, useRef, useState } from "react";
import type { main } from "../../wailsjs/go/models";

interface Props {
  worktrees: main.EditorWorktree[];
  onPick: (worktreePath: string) => void;
  onCancel: () => void;
}

/** Choose which of a context's ready worktrees to open the editor into. */
export function WorkspacePicker({ worktrees, onPick, onCancel }: Props) {
  const [selected, setSelected] = useState(worktrees[0]?.worktreePath ?? "");
  const dialogRef = useRef<HTMLDivElement>(null);

  // Keyboard-driven: ↑/↓ move the selection, Enter opens it, Esc cancels.
  // Capture-phase window listener so the keys work regardless of what holds
  // DOM focus behind the modal, and never leak into the page underneath.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const idx = worktrees.findIndex((w) => w.worktreePath === selected);
      if (e.key === "ArrowDown") {
        e.preventDefault(); e.stopPropagation();
        setSelected(worktrees[Math.min(idx + 1, worktrees.length - 1)]?.worktreePath ?? selected);
      } else if (e.key === "ArrowUp") {
        e.preventDefault(); e.stopPropagation();
        setSelected(worktrees[Math.max(idx - 1, 0)]?.worktreePath ?? selected);
      } else if (e.key === "Enter") {
        e.preventDefault(); e.stopPropagation();
        if (selected) onPick(selected);
      } else if (e.key === "Escape") {
        e.preventDefault(); e.stopPropagation();
        onCancel();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [worktrees, selected, onPick, onCancel]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Choose a workspace"
      onClick={onCancel}
      style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,0.4)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 50 }}
    >
      <div
        ref={dialogRef}
        onClick={(e) => e.stopPropagation()}
        style={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: 8, padding: 20, maxWidth: 520, width: "90%" }}
      >
        <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 12 }}>Open which workspace?</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 6, marginBottom: 14 }}>
          {worktrees.map((w) => (
            <button
              key={w.worktreePath}
              onClick={() => setSelected(w.worktreePath)}
              onDoubleClick={() => onPick(w.worktreePath)}
              aria-selected={selected === w.worktreePath}
              style={{
                textAlign: "left", padding: "8px 10px", borderRadius: 6, cursor: "pointer",
                border: `1px solid ${selected === w.worktreePath ? "var(--accent, #6366f1)" : "var(--border)"}`,
                background: selected === w.worktreePath ? "var(--surface2)" : "transparent", color: "var(--text)",
              }}
            >
              <div style={{ fontSize: 12, fontWeight: 600 }}>{w.repoName}</div>
              <div style={{ fontSize: 11, color: "var(--text-muted)" }}>{w.branch}</div>
            </button>
          ))}
        </div>
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button className="btn-secondary" style={{ fontSize: 11, padding: "4px 12px" }} onClick={onCancel}>Cancel</button>
          <button className="btn-primary" style={{ fontSize: 11, padding: "4px 12px" }} disabled={!selected} onClick={() => onPick(selected)}>Open</button>
        </div>
      </div>
    </div>
  );
}
