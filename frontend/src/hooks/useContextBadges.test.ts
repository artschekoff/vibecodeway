import { renderHook, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { ListContexts, EventsOn } = vi.hoisted(() => ({
  ListContexts: vi.fn(),
  EventsOn: vi.fn(() => () => {}), // returns an unsubscribe fn
}));
vi.mock("../../wailsjs/go/main/App", () => ({ ListContexts }));
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn }));

import { useContextBadges } from "./useContextBadges";

beforeEach(() => {
  vi.clearAllMocks();
  EventsOn.mockReturnValue(() => {});
  ListContexts.mockResolvedValue([
    { id: "c1", unviewedCount: 3, snapshotStatus: "needs_attention" },
    { id: "c2", unviewedCount: 0, snapshotStatus: "in_progress" },
  ]);
});

describe("useContextBadges", () => {
  it("maps unviewedCount and needs_attention per context", async () => {
    const { result } = renderHook(() => useContextBadges("t1"));
    await waitFor(() => expect(result.current.get("c1")).toEqual({ unviewedCount: 3, needsAttention: true }));
    expect(result.current.get("c2")).toEqual({ unviewedCount: 0, needsAttention: false });
  });

  it("refetches when the active tab changes", async () => {
    const { rerender } = renderHook(({ id }) => useContextBadges(id), { initialProps: { id: "t1" } });
    await waitFor(() => expect(ListContexts).toHaveBeenCalledTimes(1));
    rerender({ id: "t2" });
    await waitFor(() => expect(ListContexts).toHaveBeenCalledTimes(2));
  });

  it("subscribes to bg-refresh:contexts", async () => {
    renderHook(() => useContextBadges("t1"));
    await waitFor(() => expect(EventsOn).toHaveBeenCalledWith("bg-refresh:contexts", expect.any(Function)));
  });
});
