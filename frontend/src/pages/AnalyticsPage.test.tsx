import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

vi.mock("../../wailsjs/go/main/App", () => ({
  GetAnalyticsDashboard: vi.fn(),
}));

import { GetAnalyticsDashboard } from "../../wailsjs/go/main/App";
import AnalyticsPage from "./AnalyticsPage";

const mockDashboard = GetAnalyticsDashboard as unknown as ReturnType<typeof vi.fn>;

function card(metric: string, period: string, count: number, prev: number, deltaPct: number) {
  return { metric, period, count, prev, deltaPct };
}

const baseDashboard = {
  rangeDays: 30,
  cards: [
    card("tasksClosed", "yesterday", 2, 1, 100),
    card("tasksClosed", "week", 10, 8, 25),
    card("tasksClosed", "month", 40, 40, 0),
    card("prsMerged", "yesterday", 1, 2, -50),
    card("prsMerged", "week", 5, 5, 0),
    card("prsMerged", "month", 20, 15, 33.3),
    card("comments", "yesterday", 3, 0, 100),
    card("comments", "week", 12, 10, 20),
    card("comments", "month", 50, 60, -16.7),
  ],
  daily: [
    { day: "2026-08-15", tasks: 1, prs: 0, comments: 2 },
    { day: "2026-08-16", tasks: 2, prs: 1, comments: 1 },
    { day: "2026-08-17", tasks: 0, prs: 1, comments: 3 },
  ],
  byType: [
    { label: "Bug", count: 5 },
    { label: "Task", count: 3 },
  ],
  byOrg: [
    { label: "acme", count: 6 },
    { label: "globex", count: 2 },
  ],
  avgTask: {
    medianHours: 3.4,
    meanHours: 3.9,
    samples: 27,
    assumed: false,
  },
};

beforeEach(() => {
  vi.clearAllMocks();
  mockDashboard.mockResolvedValue(baseDashboard);
});

describe("AnalyticsPage", () => {
  it("renders 9 cards from a stub payload", async () => {
    render(<AnalyticsPage />);
    await waitFor(() => expect(screen.getByTestId("analytics-card-tasksClosed-yesterday")).toBeInTheDocument());

    const metrics = ["tasksClosed", "prsMerged", "comments"];
    const periods = ["yesterday", "week", "month"];
    for (const metric of metrics) {
      for (const period of periods) {
        expect(screen.getByTestId(`analytics-card-${metric}-${period}`)).toBeInTheDocument();
      }
    }
  });

  it("calls the binding with 7 when the 7d range button is clicked", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    render(<AnalyticsPage />);
    await waitFor(() => expect(mockDashboard).toHaveBeenCalledWith(30));

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "7d" }));

    await waitFor(() => expect(mockDashboard).toHaveBeenCalledWith(7));
  });

  it("renders the empty-state line when every card count and daily bucket is 0", async () => {
    mockDashboard.mockResolvedValue({
      ...baseDashboard,
      cards: baseDashboard.cards.map((c) => ({ ...c, count: 0, prev: 0, deltaPct: 0 })),
      daily: [
        { day: "2026-08-15", tasks: 0, prs: 0, comments: 0 },
        { day: "2026-08-16", tasks: 0, prs: 0, comments: 0 },
      ],
    });
    render(<AnalyticsPage />);
    await waitFor(() =>
      expect(screen.getByText(/history starts now/i)).toBeInTheDocument()
    );
    expect(screen.queryByTestId("analytics-card-tasksClosed-yesterday")).not.toBeInTheDocument();
    // Headline + refresh stay visible.
    expect(screen.getByText("Analytics")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /refresh/i })).toBeInTheDocument();
  });

  it("renders '(assumed' when avgTask.assumed is true", async () => {
    mockDashboard.mockResolvedValue({
      ...baseDashboard,
      avgTask: { medianHours: 2, meanHours: 2, samples: 1, assumed: true },
    });
    render(<AnalyticsPage />);
    await waitFor(() =>
      expect(screen.getByText(/\(assumed/)).toBeInTheDocument()
    );
  });
});
