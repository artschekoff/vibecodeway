import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListContextsByOrg: vi.fn(),
  ListEvents: vi.fn(),
  ListWorkflowStatuses: vi.fn(),
  GetContextRelations: vi.fn(),
}));

import {
  ListContextsByOrg, ListEvents, ListWorkflowStatuses, GetContextRelations,
} from "../../wailsjs/go/main/App";
import { useTimelineData } from "./useTimelineData";

const mocked = {
  ListContextsByOrg: ListContextsByOrg as unknown as ReturnType<typeof vi.fn>,
  ListEvents: ListEvents as unknown as ReturnType<typeof vi.fn>,
  ListWorkflowStatuses: ListWorkflowStatuses as unknown as ReturnType<typeof vi.fn>,
  GetContextRelations: GetContextRelations as unknown as ReturnType<typeof vi.fn>,
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useTimelineData", () => {
  it("builds lanes from active contexts and filters relations to visible targets", async () => {
    mocked.ListContextsByOrg.mockResolvedValue([
      { id: "c1", key: "auth", status: "active", snapshotStatus: "needs_attention" },
      { id: "c2", key: "db", status: "active", snapshotStatus: "in_progress" },
      { id: "c3", key: "old", status: "archived", snapshotStatus: "ready" },
    ]);
    mocked.ListWorkflowStatuses.mockResolvedValue([
      { slug: "needs_attention", label: "Needs Attention", color: "#f0a500" },
      { slug: "in_progress", label: "In Progress", color: "#4a90d9" },
    ]);
    mocked.ListEvents.mockImplementation(async (id: string) =>
      id === "c1"
        ? [{ id: "e1", type: "status_update", timestamp: "2026-07-13T10:00:00", sourceType: "jira", author: "a", summary: "s" }]
        : [],
    );
    mocked.GetContextRelations.mockImplementation(async (id: string) =>
      id === "c1"
        ? [
            { id: "r1", contextId: "c1", targetId: "c2", targetKey: "db", relationType: "blocked-by" },
            { id: "r2", contextId: "c1", targetId: "c3", targetKey: "old", relationType: "relates-to" },
          ]
        : [],
    );

    const { result } = renderHook(() => useTimelineData("org1"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.lanes.map((l) => l.contextId)).toEqual(["c1", "c2"]); // archived excluded
    expect(result.current.lanes[0].status).toBe("needs_attention");
    expect(result.current.lanes[0].events).toHaveLength(1);
    // r2 points at archived c3 (not a lane) -> dropped; r1 kept
    expect(result.current.relations).toHaveLength(1);
    expect(result.current.relations[0].targetContextId).toBe("c2");
    expect(result.current.statuses["in_progress"].label).toBe("In Progress");
  });

  it("sets a fallback error message when loading throws", async () => {
    mocked.ListContextsByOrg.mockRejectedValue(new Error("boom"));
    mocked.ListWorkflowStatuses.mockResolvedValue([]);
    const { result } = renderHook(() => useTimelineData("org1"));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe("boom");
  });
});
