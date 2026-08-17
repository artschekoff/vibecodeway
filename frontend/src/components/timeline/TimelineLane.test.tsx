import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import TimelineLane from "./TimelineLane";
import { buildDayColumns, defaultAnchor, toLaneEvent, DAY_COL_WIDTH, type Lane } from "../../lib/timeline";

const today = new Date(2026, 6, 13);
const columns = buildDayColumns(defaultAnchor(today), today);

const lane: Lane = {
  contextId: "c1",
  contextKey: "auth-service",
  status: "needs_attention",
  events: [
    toLaneEvent({ id: "e1", type: "status_update", timestamp: "2026-07-13T10:00:00", sourceType: "jira", author: "alice", summary: "moved to blocked" }),
    toLaneEvent({ id: "e2", type: "work_completed", timestamp: "2026-07-12T09:00:00", sourceType: "github", author: "bob", summary: "pushed 3 commits" }),
  ],
};

describe("TimelineLane", () => {
  it("renders the context key, status label and its in-window events", () => {
    render(<TimelineLane lane={lane} columns={columns} statusLabel="Needs Attention" statusColor="#f0a500" />);
    expect(screen.getByText("auth-service")).toBeInTheDocument();
    expect(screen.getByText("Needs Attention")).toBeInTheDocument();
    expect(screen.getByText("moved to blocked")).toBeInTheDocument();
    expect(screen.getByText("pushed 3 commits")).toBeInTheDocument();
  });

  it("prefixes each event row with its own day", () => {
    render(<TimelineLane lane={lane} columns={columns} statusLabel="Needs Attention" statusColor="#f0a500" />);
    expect(screen.getByText("13.07")).toBeInTheDocument();
    expect(screen.getByText("12.07")).toBeInTheDocument();
  });

  it("opens the context when the lane key is clicked", () => {
    const onOpen = vi.fn();
    render(<TimelineLane lane={lane} columns={columns} statusLabel="Needs Attention" statusColor="#f0a500" onOpen={onOpen} />);
    fireEvent.click(screen.getByRole("button", { name: "auth-service" }));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it("starts each event row under its own day column", () => {
    render(<TimelineLane lane={lane} columns={columns} statusLabel="Needs Attention" statusColor="#f0a500" />);
    // window is 07.07–13.07: 13.07 is column 6, 12.07 is column 5.
    const row = (text: string) => screen.getByText(text).closest(".timeline-event-row") as HTMLElement;
    expect(row("moved to blocked").style.marginLeft).toBe(`${6 * DAY_COL_WIDTH}px`);
    expect(row("pushed 3 commits").style.marginLeft).toBe(`${5 * DAY_COL_WIDTH}px`);
  });
});
