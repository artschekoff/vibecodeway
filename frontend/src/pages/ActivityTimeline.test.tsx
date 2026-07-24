import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";

vi.mock("../../wailsjs/go/main/App", () => ({
  ListOrganizations: vi.fn(),
  ListContextsByOrg: vi.fn(),
  ListEvents: vi.fn(),
  ListWorkflowStatuses: vi.fn(),
  GetContextRelations: vi.fn(),
}));

import {
  ListOrganizations, ListContextsByOrg, ListEvents, ListWorkflowStatuses, GetContextRelations,
} from "../../wailsjs/go/main/App";
import ActivityTimeline from "./ActivityTimeline";

const m = {
  ListOrganizations: ListOrganizations as unknown as ReturnType<typeof vi.fn>,
  ListContextsByOrg: ListContextsByOrg as unknown as ReturnType<typeof vi.fn>,
  ListEvents: ListEvents as unknown as ReturnType<typeof vi.fn>,
  ListWorkflowStatuses: ListWorkflowStatuses as unknown as ReturnType<typeof vi.fn>,
  GetContextRelations: GetContextRelations as unknown as ReturnType<typeof vi.fn>,
};

beforeEach(() => {
  vi.clearAllMocks();
  m.ListOrganizations.mockResolvedValue([{ id: "org1", name: "Acme" }]);
  m.ListWorkflowStatuses.mockResolvedValue([{ slug: "in_progress", label: "In Progress", color: "#4a90d9" }]);
  m.ListContextsByOrg.mockResolvedValue([{ id: "c1", key: "auth", status: "active", snapshotStatus: "in_progress" }]);
  m.ListEvents.mockResolvedValue([]);
  m.GetContextRelations.mockResolvedValue([]);
});

describe("ActivityTimeline", () => {
  it("renders the org lanes and a 7-column day header", async () => {
    render(<ActivityTimeline />);
    await waitFor(() => expect(screen.getByText("auth")).toBeInTheDocument());
    // 7 day-header cells
    expect(screen.getAllByTestId("timeline-day-header")).toHaveLength(7);
  });

  it("prev/next/today buttons are present and shift the window", async () => {
    render(<ActivityTimeline />);
    await waitFor(() => expect(screen.getByText("auth")).toBeInTheDocument());
    const headersBefore = screen.getAllByTestId("timeline-day-header").map((n) => n.textContent);
    fireEvent.click(screen.getByRole("button", { name: "‹ Back" }));
    const headersAfter = screen.getAllByTestId("timeline-day-header").map((n) => n.textContent);
    expect(headersAfter).not.toEqual(headersBefore);
    // Today resets
    fireEvent.click(screen.getByRole("button", { name: "Today" }));
    const headersReset = screen.getAllByTestId("timeline-day-header").map((n) => n.textContent);
    expect(headersReset).toEqual(headersBefore);
  });

  it("renders a dependency connector for a relation between visible lanes", async () => {
    m.ListContextsByOrg.mockResolvedValue([
      { id: "c1", key: "auth", status: "active", snapshotStatus: "in_progress" },
      { id: "c2", key: "db", status: "active", snapshotStatus: "in_progress" },
    ]);
    m.GetContextRelations.mockImplementation(async (id: string) =>
      id === "c1"
        ? [{ id: "r1", contextId: "c1", targetId: "c2", targetKey: "db", relationType: "blocked-by" }]
        : [],
    );
    const { container } = render(<ActivityTimeline />);
    await waitFor(() => expect(screen.getByText("auth")).toBeInTheDocument());
    await waitFor(() => {
      const titles = Array.from(container.querySelectorAll(".timeline-connectors title")).map((t) => t.textContent);
      expect(titles).toContain("blocked-by → db");
    });
  });
});
