import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

vi.mock("../../wailsjs/go/main/App", () => ({
  GetProductivityStats: vi.fn(),
}));

import { GetProductivityStats } from "../../wailsjs/go/main/App";
import ProductivityPage from "./ProductivityPage";

const mockStats = GetProductivityStats as unknown as ReturnType<typeof vi.fn>;

const baseStats = {
  today: 2,
  yesterday: 3,
  weekAvg: 1.5,
  monthAvg: 0.9,
  daily: [
    { day: "2026-08-15", count: 1 },
    { day: "2026-08-16", count: 3 },
    { day: "2026-08-17", count: 2 },
  ],
  workdayHours: 8,
  taskHours: 2,
  hoursToday: 4,
  tasksToNorm: 2,
  tasksToBeatYesterday: 2,
  tasksToBeatWeek: 0,
  tasksToBeatMonth: 0,
};

beforeEach(() => {
  vi.clearAllMocks();
  mockStats.mockResolvedValue(baseStats);
});

describe("ProductivityPage", () => {
  it("renders the headline sentence from the stats", async () => {
    render(<ProductivityPage />);
    await waitFor(() =>
      expect(
        screen.getByText(
          "You closed 2 tasks today — 2h each, 4h of 8h. 2 more tasks to close the day."
        )
      ).toBeInTheDocument()
    );
  });

  it("shows how many tasks are needed to beat each baseline", async () => {
    render(<ProductivityPage />);
    await waitFor(() => expect(screen.getByTestId("baseline-yesterday")).toBeInTheDocument());
    expect(screen.getByTestId("baseline-yesterday")).toHaveTextContent("2 more to beat it");
    expect(screen.getByTestId("baseline-week")).toHaveTextContent("Ahead");
    expect(screen.getByTestId("baseline-month")).toHaveTextContent("Ahead");
  });

  it("renders one bar per day in the series", async () => {
    render(<ProductivityPage />);
    await waitFor(() => expect(screen.getAllByTestId("productivity-bar")).toHaveLength(3));
  });

  it("says the norm is met when nothing is left", async () => {
    mockStats.mockResolvedValue({ ...baseStats, today: 4, hoursToday: 8, tasksToNorm: 0 });
    render(<ProductivityPage />);
    await waitFor(() =>
      expect(
        screen.getByText("You closed 4 tasks today — 2h each, 8h of 8h. Workday norm met.")
      ).toBeInTheDocument()
    );
  });

  it("shows an empty-history explanation instead of the headline on a fresh database", async () => {
    mockStats.mockResolvedValue({
      ...baseStats,
      today: 0,
      hoursToday: 0,
      tasksToNorm: 4,
      tasksToBeatYesterday: 0,
      tasksToBeatWeek: 0,
      tasksToBeatMonth: 0,
      daily: [
        { day: "2026-08-15", count: 0 },
        { day: "2026-08-16", count: 0 },
        { day: "2026-08-17", count: 0 },
      ],
    });
    render(<ProductivityPage />);
    await waitFor(() =>
      expect(screen.getByText(/history starts now/i)).toBeInTheDocument()
    );
    expect(screen.queryByText(/You closed 0 tasks today/)).not.toBeInTheDocument();
    // Cards and bars stay visible below the explanation.
    expect(screen.getByTestId("baseline-yesterday")).toBeInTheDocument();
    expect(screen.getAllByTestId("productivity-bar")).toHaveLength(3);
  });
});
