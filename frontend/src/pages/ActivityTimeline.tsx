import { useEffect, useMemo, useRef, useState } from "react";
import { ListOrganizations } from "../../wailsjs/go/main/App";
import {
  addDays,
  buildDayColumns,
  defaultAnchor,
  DAY_COL_WIDTH,
  LABEL_COL_WIDTH,
  LANE_HEIGHT,
  WINDOW_DAYS,
} from "../lib/timeline";
import { useTimelineData } from "../hooks/useTimelineData";
import TimelineLane from "../components/timeline/TimelineLane";

interface Org {
  id: string;
  name: string;
}

export default function ActivityTimeline() {
  const [orgs, setOrgs] = useState<Org[]>([]);
  const [orgId, setOrgId] = useState<string>("");
  // `today` is captured once so the window is stable across re-renders.
  const todayRef = useRef<Date>(new Date());
  const [anchor, setAnchor] = useState<Date>(() => defaultAnchor(todayRef.current));

  useEffect(() => {
    let cancelled = false;
    ListOrganizations()
      .then((list) => {
        if (cancelled) return;
        const os = (list ?? []).map((o) => ({ id: o.id, name: o.name }));
        setOrgs(os);
        setOrgId((cur) => cur || os[0]?.id || "");
      })
      .catch(() => {
        /* org list unavailable — page shows empty state */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const { lanes, relations, statuses, loading, error } = useTimelineData(orgId);
  const columns = useMemo(
    () => buildDayColumns(anchor, todayRef.current, WINDOW_DAYS),
    [anchor],
  );

  // lane row index for connector alignment
  const laneIndex = useMemo(() => {
    const map = new Map<string, number>();
    lanes.forEach((l, i) => map.set(l.contextId, i));
    return map;
  }, [lanes]);

  const gutterHeight = lanes.length * LANE_HEIGHT;

  return (
    <div className="timeline-page">
      {/* toolbar */}
      <div className="timeline-toolbar">
        <select
          className="form-control timeline-org-select"
          value={orgId}
          onChange={(e) => setOrgId(e.target.value)}
        >
          {orgs.map((o) => (
            <option key={o.id} value={o.id}>{o.name}</option>
          ))}
        </select>
        <div className="timeline-nav">
          <button className="btn-ghost" onClick={() => setAnchor((a) => addDays(a, -WINDOW_DAYS))}>‹ Back</button>
          <button className="btn-ghost" onClick={() => setAnchor(defaultAnchor(todayRef.current))}>Today</button>
          <button className="btn-ghost" onClick={() => setAnchor((a) => addDays(a, WINDOW_DAYS))}>Forward ›</button>
        </div>
        {error && <span className="timeline-error">{error}</span>}
      </div>

      {/* scrollable timeline body */}
      <div className="timeline-scroll">
        <div
          className="timeline-grid"
          style={{ minWidth: LABEL_COL_WIDTH + columns.length * DAY_COL_WIDTH }}
        >
          {/* day header */}
          <div className="timeline-header-row" style={{ height: 40 }}>
            <div className="timeline-corner" style={{ width: LABEL_COL_WIDTH }} />
            {columns.map((col) => (
              <div
                key={col.key}
                data-testid="timeline-day-header"
                className={"timeline-day-header" + (col.isToday ? " is-today" : "")}
                style={{ width: DAY_COL_WIDTH }}
              >
                {col.label}
              </div>
            ))}
          </div>

          {/* lanes + connectors overlay */}
          <div className="timeline-lanes" style={{ position: "relative" }}>
            {/* connectors live in the sticky left gutter, aligned to lane index */}
            <svg
              className="timeline-connectors"
              width={LABEL_COL_WIDTH}
              height={gutterHeight}
              style={{ position: "absolute", left: 0, top: 0, pointerEvents: "none" }}
            >
              <defs>
                <marker id="tl-arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
                  <path d="M0,0 L6,3 L0,6 Z" fill="var(--accent)" />
                </marker>
              </defs>
              {relations.map((rel) => {
                const from = laneIndex.get(rel.sourceContextId);
                const to = laneIndex.get(rel.targetContextId);
                if (from === undefined || to === undefined) return null;
                const y1 = from * LANE_HEIGHT + LANE_HEIGHT / 2;
                const y2 = to * LANE_HEIGHT + LANE_HEIGHT / 2;
                const x = LABEL_COL_WIDTH - 10;
                const bow = Math.min(60, 20 + Math.abs(to - from) * 12);
                return (
                  <path
                    key={rel.id}
                    d={`M ${x} ${y1} C ${x - bow} ${y1}, ${x - bow} ${y2}, ${x} ${y2}`}
                    stroke="var(--accent)"
                    strokeWidth={2}
                    fill="none"
                    markerEnd="url(#tl-arrow)"
                  >
                    <title>{`${rel.relationType} → ${rel.targetKey}`}</title>
                  </path>
                );
              })}
            </svg>

            {loading ? (
              <div className="timeline-loading">Loading…</div>
            ) : lanes.length === 0 ? (
              <div className="timeline-loading">No active contexts in this organization</div>
            ) : (
              lanes.map((lane) => {
                const info = statuses[lane.status];
                return (
                  <TimelineLane
                    key={lane.contextId}
                    lane={lane}
                    columns={columns}
                    statusLabel={info?.label ?? lane.status}
                    statusColor={info?.color ?? ""}
                  />
                );
              })
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
