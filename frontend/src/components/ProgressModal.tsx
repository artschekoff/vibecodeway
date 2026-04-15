import { useProgress } from "../contexts/ProgressContext";

export default function ProgressModal() {
  const { isActive, title, items, currentIndex, current, total, stepText, cancellable, cancel } = useProgress();

  if (!isActive) return null;

  const pct = total > 0 ? Math.round((current / total) * 100) : 0;
  const isIndeterminate = total === 0 || (current === 0 && currentIndex < 0);

  return (
    <div className="modal-overlay">
      <div className="modal progress-modal">
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
                    {isDone ? "✓" : isCurrent ? "›" : "·"}
                  </span>
                  <span className="progress-item-label">{label}</span>
                </div>
              );
            })}
          </div>
        )}

        {/* Counter */}
        {total > 0 && (
          <div className="progress-counter">
            {stepText && <span className="progress-step-text">{stepText}</span>}
            <span className="progress-count-badge">{current} / {total}</span>
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

        {/* Step hint — shown only when total is not yet known (event-driven mode, starting) */}
        {items.length === 0 && total === 0 && (
          <p className="reload-progress-hint">{stepText}</p>
        )}

        {cancellable && (
          <div className="modal-footer" style={{ marginTop: 16 }}>
            <button className="btn-secondary" onClick={cancel}>Cancel</button>
          </div>
        )}
      </div>
    </div>
  );
}
