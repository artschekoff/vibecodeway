// Pure date-axis + event-grouping utilities for the Activity Timeline.
// No external deps: native Date + deterministic Russian labels (no Intl locale).

export const WINDOW_DAYS = 7;
export const DAY_COL_WIDTH = 150; // px per day column
export const LANE_HEIGHT = 116; // px, FIXED — relation connectors align to lane index
export const LABEL_COL_WIDTH = 190; // px, sticky left column

const RU_WEEKDAYS = ["Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"];

export interface DayColumn {
  key: string; // local YYYY-MM-DD
  date: Date; // local midnight
  label: string; // "Пн 13.07"
  isToday: boolean;
}

export interface LaneEvent {
  id: string;
  type: string;
  timestamp: string; // ISO
  sourceType: string; // jira | slack | github | chat | user
  author: string;
  summary: string;
  dayKey: string; // derived local day
}

export interface Lane {
  contextId: string;
  contextKey: string;
  status: string; // workflow status slug
  events: LaneEvent[]; // ascending by timestamp
}

export interface RelationEdge {
  id: string;
  sourceContextId: string;
  targetContextId: string;
  targetKey: string;
  relationType: string;
}

export function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

export function addDays(d: Date, n: number): Date {
  const r = startOfDay(d);
  r.setDate(r.getDate() + n);
  return r;
}

export function dayKey(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

export function isoToDayKey(iso: string): string {
  return dayKey(new Date(iso));
}

export function formatDayLabel(d: Date): string {
  const wd = RU_WEEKDAYS[d.getDay()];
  const dd = String(d.getDate()).padStart(2, "0");
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  return `${wd} ${dd}.${mm}`;
}

export function defaultAnchor(today: Date, windowDays: number = WINDOW_DAYS): Date {
  // leftmost day chosen so `today` is the rightmost visible column
  return addDays(today, -(windowDays - 1));
}

export function buildDayColumns(
  anchor: Date,
  today: Date,
  windowDays: number = WINDOW_DAYS,
): DayColumn[] {
  const todayKey = dayKey(today);
  const cols: DayColumn[] = [];
  for (let i = 0; i < windowDays; i++) {
    const date = addDays(anchor, i);
    const key = dayKey(date);
    cols.push({ key, date, label: formatDayLabel(date), isToday: key === todayKey });
  }
  return cols;
}

export function toLaneEvent(e: {
  id: string;
  type: string;
  timestamp: string;
  sourceType: string;
  author: string;
  summary: string;
}): LaneEvent {
  return {
    id: e.id,
    type: e.type,
    timestamp: e.timestamp,
    sourceType: e.sourceType,
    author: e.author,
    summary: e.summary,
    dayKey: isoToDayKey(e.timestamp),
  };
}

export function eventsInWindow(events: LaneEvent[], columns: DayColumn[]): LaneEvent[] {
  const keys = new Set(columns.map((c) => c.key));
  return events.filter((e) => keys.has(e.dayKey));
}

export function sourceColorVar(sourceType: string): string {
  switch (sourceType) {
    case "jira":
      return "#4a90d9";
    case "github":
      return "#a371f7";
    case "slack":
      return "#4caf7d";
    case "user":
      return "var(--accent)";
    default:
      return "var(--text-muted)";
  }
}
