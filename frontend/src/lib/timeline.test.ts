import { describe, it, expect } from "vitest";
import {
  startOfDay, addDays, dayKey, isoToDayKey, formatDayLabel,
  defaultAnchor, buildDayColumns, toLaneEvent, eventsInWindow, sourceColorVar,
  WINDOW_DAYS,
} from "./timeline";

describe("startOfDay / addDays", () => {
  it("strips the time component", () => {
    const d = new Date(2026, 6, 13, 15, 42, 9);
    const s = startOfDay(d);
    expect(s.getHours()).toBe(0);
    expect(s.getMinutes()).toBe(0);
    expect(s.getDate()).toBe(13);
  });
  it("addDays crosses a month boundary", () => {
    const d = new Date(2026, 6, 31); // 31 Jul 2026
    const r = addDays(d, 1);
    expect(r.getMonth()).toBe(7); // August
    expect(r.getDate()).toBe(1);
  });
  it("addDays with negative n goes back", () => {
    const r = addDays(new Date(2026, 6, 1), -1);
    expect(r.getMonth()).toBe(5); // June
    expect(r.getDate()).toBe(30);
  });
});

describe("dayKey / isoToDayKey", () => {
  it("zero-pads month and day", () => {
    expect(dayKey(new Date(2026, 0, 5))).toBe("2026-01-05");
  });
  it("isoToDayKey derives the local day from a timestamp", () => {
    expect(isoToDayKey("2026-07-13T09:30:00")).toBe("2026-07-13");
  });
});

describe("formatDayLabel", () => {
  it("renders a Russian weekday abbrev + dd.mm", () => {
    // 2026-07-13 is a Monday
    expect(formatDayLabel(new Date(2026, 6, 13))).toBe("Пн 13.07");
    // 2026-07-12 is a Sunday
    expect(formatDayLabel(new Date(2026, 6, 12))).toBe("Вс 12.07");
  });
});

describe("defaultAnchor / buildDayColumns", () => {
  it("defaultAnchor places today as the rightmost column", () => {
    const today = new Date(2026, 6, 13);
    const anchor = defaultAnchor(today);
    expect(dayKey(anchor)).toBe("2026-07-07"); // today - 6
  });
  it("buildDayColumns returns WINDOW_DAYS columns with today flagged", () => {
    const today = new Date(2026, 6, 13);
    const cols = buildDayColumns(defaultAnchor(today), today);
    expect(cols).toHaveLength(WINDOW_DAYS);
    expect(cols[0].key).toBe("2026-07-07");
    expect(cols[6].key).toBe("2026-07-13");
    expect(cols[6].isToday).toBe(true);
    expect(cols[0].isToday).toBe(false);
    expect(cols[0].label).toBe("Вт 07.07");
  });
});

describe("toLaneEvent / eventsInWindow", () => {
  const raw = {
    id: "e1", type: "status_update", timestamp: "2026-07-13T10:00:00",
    sourceType: "jira", author: "alice", summary: "moved to in progress",
  };
  it("toLaneEvent derives dayKey from timestamp", () => {
    expect(toLaneEvent(raw).dayKey).toBe("2026-07-13");
  });
  it("eventsInWindow keeps only events whose day is in the columns", () => {
    const today = new Date(2026, 6, 13);
    const cols = buildDayColumns(defaultAnchor(today), today);
    const inside = toLaneEvent(raw);
    const outside = toLaneEvent({ ...raw, id: "e2", timestamp: "2026-06-01T10:00:00" });
    expect(eventsInWindow([inside, outside], cols)).toEqual([inside]);
  });
});

describe("sourceColorVar", () => {
  it("maps known sources and falls back for unknown", () => {
    expect(sourceColorVar("github")).toBe("#a371f7");
    expect(sourceColorVar("jira")).toBe("#4a90d9");
    expect(sourceColorVar("slack")).toBe("#4caf7d");
    expect(sourceColorVar("user")).toBe("var(--accent)");
    expect(sourceColorVar("whatever")).toBe("var(--text-muted)");
  });
});
