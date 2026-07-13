import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import TimelineLane from "./TimelineLane";
import { buildDayColumns, defaultAnchor, toLaneEvent, type Lane } from "../../lib/timeline";

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
});
