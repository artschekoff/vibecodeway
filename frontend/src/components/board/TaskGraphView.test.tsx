import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const H = vi.hoisted(() => ({
  GetTaskGraph: vi.fn(),
  RefreshTaskGraph: vi.fn(),
  ListTaskProviders: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => H);
vi.mock("../../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL: vi.fn(), EventsOn: vi.fn(() => () => {}) }));

import { TaskGraphView } from "./TaskGraphView";

const node = (key: string, p: Record<string, unknown> = {}) => ({
  key, summary: `${key} work`, status: "To Do", type: "Task", priority: "High", priorityRank: 4,
  assignee: "", parentKey: "", mine: true, terminal: false, epic: false, blocked: false, ...p,
});
const graph = {
  provider: "jira",
  nodes: [
    node("E-1", { epic: true, type: "Epic", mine: false }),
    node("T-1", { blocked: true, blockedBy: ["X-1"], priorityRank: 5, priority: "Highest" }),
    node("T-2", { contextId: "c1", contextKey: "ctx-two" }),
    node("X-1", { mine: false, status: "In Progress" }),
  ],
  edges: [
    { src: "T-1", dst: "E-1", kind: "parent" },
    { src: "T-2", dst: "E-1", kind: "parent" },
    { src: "X-1", dst: "T-1", kind: "blocks" },
  ],
  ready: ["T-2"],
  waiting: ["T-1"],
};

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  H.ListTaskProviders.mockResolvedValue(["jira"]);
  H.GetTaskGraph.mockResolvedValue(graph);
  H.RefreshTaskGraph.mockResolvedValue({ ...graph, ready: ["T-2", "T-1"], waiting: [], nodes: graph.nodes.map((n) => ({ ...n, blocked: false })) });
});

function renderView(extra: Partial<React.ComponentProps<typeof TaskGraphView>> = {}) {
  const onOpenContext = vi.fn();
  render(<TaskGraphView onOpenContext={onOpenContext} onCreateContext={vi.fn()} orgs={[]} selectedOrgId="" onSelectOrg={vi.fn()} creatingKey="" reloadToken={0} {...extra} />);
  return { onOpenContext };
}

describe("TaskGraphView", () => {
  it("renders the graph, ready/waiting rails, and marks blocked nodes", async () => {
    renderView();
    await waitFor(() => expect(screen.getByTestId("task-graph").querySelectorAll("[data-key]").length).toBe(4));
    expect(screen.getByTestId("task-graph").querySelector('[data-key="T-1"][data-blocked]')).not.toBeNull();
    expect(screen.getByText("Ready now").parentElement).toHaveTextContent("1");
    expect(screen.getByText("X-1 (In Progress)")).toBeInTheDocument(); // waiting row note
  });

  it("clicking a node shows details with blockers, parent chain and the context button", async () => {
    const { onOpenContext } = renderView();
    await waitFor(() => expect(screen.getByRole("button", { name: "T-1 T-1 work" })).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "T-1 T-1 work" }));
    expect(screen.getByText(/Blocked by X-1 \(In Progress\)/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "E-1 (epic)" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "T-2 T-2 work" }));
    fireEvent.click(screen.getByRole("button", { name: /open context ctx-two/i }));
    expect(onOpenContext).toHaveBeenCalledWith("c1", "ctx-two");
  });

  it("Refresh pulls from the tracker and announces newly unblocked issues", async () => {
    const onRankChange = vi.fn();
    renderView({ onRankChange });
    await waitFor(() => expect(onRankChange).toHaveBeenCalledWith(["T-2"], ["T-1"]));
    fireEvent.click(screen.getByRole("button", { name: /refresh/i }));
    expect(H.RefreshTaskGraph).toHaveBeenCalledWith("jira", false);
    expect(await screen.findByRole("status")).toHaveTextContent("Unblocked: T-1");
    expect(onRankChange).toHaveBeenLastCalledWith(["T-2", "T-1"], []);
  });
});
