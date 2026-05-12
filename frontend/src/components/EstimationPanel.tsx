import { useState } from "react";
import type { ContextSnapshot, EstimationLog } from "../types";

interface Props {
  snapshot: ContextSnapshot;
  history: EstimationLog[];
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatDelta(delta: number): string {
  return delta >= 0 ? `+${delta.toFixed(1)}h` : `${delta.toFixed(1)}h`;
}

export default function EstimationPanel({ snapshot, history }: Props) {
  const [historyOpen, setHistoryOpen] = useState(false);

  if (!snapshot.estimatedHours) {
    return (
      <div className="estimation-panel estimation-panel--empty">
        <span className="estimation-panel__empty-label">No estimate yet</span>
      </div>
    );
  }

  return (
    <div className="estimation-panel">
      <div className="estimation-panel__header">
        <span className="estimation-panel__hours">{snapshot.estimatedHours.toFixed(1)}h remaining</span>
        {snapshot.estimationConfidence && (
          <span className={`estimation-panel__confidence estimation-panel__confidence--${snapshot.estimationConfidence}`}>
            {snapshot.estimationConfidence}
          </span>
        )}
        {snapshot.estimatedCompletionDate && (
          <span className="estimation-panel__eta">
            ETA: {formatDate(snapshot.estimatedCompletionDate)}
          </span>
        )}
      </div>
      {snapshot.estimationBasis && (
        <p className="estimation-panel__basis">{snapshot.estimationBasis}</p>
      )}
      {history.length > 0 && (
        <div className="estimation-panel__history">
          <button
            className="estimation-panel__history-toggle"
            onClick={() => setHistoryOpen((o) => !o)}
            aria-label={historyOpen ? "Collapse history" : "Show history"}
          >
            History ({history.length})
          </button>
          {historyOpen && (
            <ul className="estimation-panel__history-list">
              {history.map((entry) => (
                <li key={entry.id} className="estimation-panel__history-entry">
                  <span className="estimation-panel__history-type">{entry.triggerType}</span>
                  {entry.deltaHours !== undefined && (
                    <span
                      className={`estimation-panel__history-delta ${
                        entry.deltaHours >= 0
                          ? "estimation-panel__history-delta--positive"
                          : "estimation-panel__history-delta--negative"
                      }`}
                    >
                      {formatDelta(entry.deltaHours)}
                    </span>
                  )}
                  {entry.estimatedHoursAfter !== undefined && (
                    <span className="estimation-panel__history-total">
                      → {entry.estimatedHoursAfter.toFixed(1)}h
                    </span>
                  )}
                  <span className="estimation-panel__history-date">
                    {formatDate(entry.recordedAt)}
                  </span>
                  {entry.estimationBasis && (
                    <span className="estimation-panel__history-basis">{entry.estimationBasis}</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
