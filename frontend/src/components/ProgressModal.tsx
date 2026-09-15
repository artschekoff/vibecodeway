import { useEffect, useState } from "react";
import { Check, ChevronRight, Loader2, Maximize2, Minimize2, Minus } from "lucide-react";
import { useProgress } from "../contexts/ProgressContext";

export default function ProgressModal() {
  const { isActive, title, items, currentIndex, current, total, stepText, subStep, cancellable, cancel } = useProgress();
  // Minimized: the operation keeps running (it's backend-driven either way);
  // only the blocking overlay collapses into a corner pill so the app stays usable.
  const [minimized, setMinimized] = useState(false);
  useEffect(() => {
    if (!isActive) setMinimized(false); // next operation starts expanded
  }, [isActive]);

  if (!isActive) return null;

  const pct = total > 0 ? Math.round((current / total) * 100) : 0;
  const isIndeterminate = total === 0 || (current === 0 && currentIndex < 0);

  if (minimized) {
    return (
      <div
        className="bg-refresh-toast"
        role="status"
        // Sits above the bg-refresh toast's spot so the two never overlap.
        style={{ bottom: 76, cursor: "pointer" }}
        onClick={() => setMinimized(false)}
        title="Click to expand"
        data-testid="progress-minimized"
      >
        <div className="bg-refresh-toast__label" style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <Loader2 size={12} className="spin" />
          <span>{title || "Processing…"}{total > 0 ? ` ${pct}%` : ""}</span>
          {subStep && <span className="bg-refresh-toast__key">{subStep}</span>}
          <Maximize2 size={11} style={{ flexShrink: 0, opacity: 0.6 }} />
        </div>
        <div className={`bg-refresh-toast__bar${isIndeterminate ? " progress-indeterminate" : ""}`}>
          <div className="bg-refresh-toast__fill" style={isIndeterminate ? undefined : { width: `${pct}%` }} />
        </div>
      </div>
    );
  }

  return (
    <div className="modal-overlay modal-overlay--progress">
      <div className="modal progress-modal">
        <button
          className="btn-ghost"
          aria-label="minimize progress"
          title="Minimize — keep running in the background"
          onClick={() => setMinimized(true)}
          style={{ position: "absolute", top: 10, right: 10, padding: 4, display: "flex" }}
        >
          <Minimize2 size={14} />
        </button>
        <h2>{title || "Processing…"}</h2>

        {/* Item list — rendered when we know items upfront */}
        {items.length > 0 && (
          <div className="progress-items">
            {items.map((label, i) => {
              const isDone = i < currentIndex;
              const isCurrent = i === currentIndex;
              return (
                <div
                  key={i}
                  className={`progress-item${isDone ? " progress-item--done" : ""}${isCurrent ? " progress-item--current" : ""}`}
                >
                  <span className="progress-item-icon">
                    {isDone ? <Check size={11} /> : isCurrent ? <ChevronRight size={11} /> : <Minus size={10} />}
                  </span>
                  <span className="progress-item-label">{label}</span>
                </div>
              );
            })}
          </div>
        )}

        {/* Detailed progress — shown in event-driven mode (no items list) */}
        {items.length === 0 && (
          <div className="progress-detail">
            {total > 0 && (
              <div className="progress-detail-header">
                <span className="progress-pct">{pct}%</span>
                <span className="progress-count">{current} / {total}</span>
              </div>
            )}

            {stepText && (
              <div className="progress-context-row">
                <Loader2 size={15} className="progress-context-icon" />
                <span className="progress-context-name">{stepText}</span>
              </div>
            )}

            {subStep && (
              <div className="progress-substep-row">
                <span className="progress-substep-dot" />
                <span className="progress-substep-text">{subStep}…</span>
              </div>
            )}

            {!stepText && !subStep && (
              <p className="reload-progress-hint">Starting…</p>
            )}
          </div>
        )}

        {/* Bar */}
        <div className="reload-progress-bar-track">
          {isIndeterminate ? (
            <div className="reload-progress-bar-fill reload-progress-bar-fill--indeterminate" />
          ) : (
            <div
              className="reload-progress-bar-fill"
              style={{ width: `${pct}%` }}
            />
          )}
        </div>

        {cancellable && (
          <div className="modal-footer" style={{ marginTop: 16 }}>
            <button className="btn-secondary" onClick={cancel}>Cancel</button>
          </div>
        )}
      </div>
    </div>
  );
}
