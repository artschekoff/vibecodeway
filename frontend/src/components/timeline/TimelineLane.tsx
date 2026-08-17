import {
  DAY_COL_WIDTH,
  LANE_HEIGHT,
  LABEL_COL_WIDTH,
  eventsInWindow,
  formatDayShort,
  sourceColorVar,
  type DayColumn,
  type Lane,
} from "../../lib/timeline";

interface Props {
  lane: Lane;
  columns: DayColumn[];
  statusLabel: string;
  statusColor: string;
  onOpen?: () => void;
}

export default function TimelineLane({ lane, columns, statusLabel, statusColor, onOpen }: Props) {
  const windowEvents = eventsInWindow(lane.events, columns);
  // Rows are a flat list; indenting each by its column index makes it start
  // under the day it happened on. Summaries still overflow to the right.
  const dayIndex = new Map(columns.map((c, i) => [c.key, i]));
  const countByDay = new Map<string, number>();
  for (const e of windowEvents) countByDay.set(e.dayKey, (countByDay.get(e.dayKey) ?? 0) + 1);

  return (
    <div className="timeline-lane" style={{ height: LANE_HEIGHT }}>
      {/* sticky left label */}
      <div className="timeline-lane-label" style={{ width: LABEL_COL_WIDTH }}>
        <button
          type="button"
          className="timeline-lane-key"
          title={`Open ${lane.contextKey}`}
          onClick={onOpen}
        >
          {lane.contextKey}
        </button>
        {statusLabel && (
          <span className="timeline-status-badge" style={{ background: statusColor || "var(--surface2)" }}>
            {statusLabel}
          </span>
        )}
      </div>

      {/* time track: day cells with event-count dots on top, scroll list below */}
      <div className="timeline-lane-track">
        <div className="timeline-lane-daygrid">
          {columns.map((col) => {
            const n = countByDay.get(col.key) ?? 0;
            return (
              <div
                key={col.key}
                className={"timeline-daycell" + (col.isToday ? " is-today" : "")}
                style={{ width: DAY_COL_WIDTH }}
              >
                {n > 0 && (
                  <span className="timeline-daycount">{n}</span>
                )}
              </div>
            );
          })}
        </div>

        <div className="timeline-lane-events">
          {windowEvents.length === 0 ? (
            <div className="timeline-empty">No events in this window</div>
          ) : (
            windowEvents.map((e) => (
              <div
                key={e.id}
                className="timeline-event-row"
                title={e.summary}
                style={{ marginLeft: (dayIndex.get(e.dayKey) ?? 0) * DAY_COL_WIDTH }}
              >
                <span className="timeline-event-dot" style={{ background: sourceColorVar(e.sourceType) }} />
                <span className="timeline-event-day">{formatDayShort(e.dayKey)}</span>
                <span className="timeline-event-summary">{e.summary}</span>
                <span className="timeline-event-meta">{e.author}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
