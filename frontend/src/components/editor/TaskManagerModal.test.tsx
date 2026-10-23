import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TaskManagerModal, groupRows } from "./TaskManagerModal";
import { registerTermSession } from "../../lib/terminalRegistry";
import { GetProcessStats } from "../../../wailsjs/go/main/App";

vi.mock("../../../wailsjs/go/main/App", () => ({
  GetProcessStats: vi.fn(),
}));

const stats = {
  app: { sessionId: "", pid: 1, cpu: 61.5, rssBytes: 900 * 1024 * 1024, procs: 12, topComm: "claude" },
  sessions: [
    { sessionId: "s1", pid: 200, cpu: 55.5, rssBytes: 500 * 1024 * 1024, procs: 3, topComm: "claude" },
    { sessionId: "s2", pid: 300, cpu: 1.0, rssBytes: 40 * 1024 * 1024, procs: 1, topComm: "zsh" },
  ],
};

describe("TaskManagerModal", () => {
  beforeEach(() => {
    vi.mocked(GetProcessStats).mockResolvedValue(stats as never);
    registerTermSession("s1", { contextKey: "ctx-a", tabLabel: "acme-api" });
    registerTermSession("s2", { contextKey: "ctx-b", tabLabel: "rdbms" });
  });

  it("groupRows groups by context and sorts by CPU", () => {
    const groups = groupRows(stats.sessions);
    expect(groups.map((g) => g.contextKey)).toEqual(["ctx-a", "ctx-b"]);
    expect(groups[0].rows[0].tabLabel).toBe("acme-api");
    expect(groups[0].cpu).toBeCloseTo(55.5);
  });

  it("renders app totals and per-tab rows; unknown sessions fall back to top process", async () => {
    render(<TaskManagerModal onClose={() => {}} />);
    expect(await screen.findByText(/app total: 61.5% CPU/)).toBeInTheDocument();
    expect(screen.getByText("acme-api")).toBeInTheDocument();
    expect(screen.getByText("ctx-b")).toBeInTheDocument();
    // context-total row and its single tab row both show the same memory
    expect(screen.getAllByText("500 MB")).toHaveLength(2);
  });

  it("clicking a row toggles a details line with the full names", async () => {
    render(<TaskManagerModal onClose={() => {}} />);
    await screen.findByText(/app total/);
    expect(screen.queryByText("acme-api · claude")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("acme-api"));
    expect(screen.getByText("acme-api · claude")).toBeInTheDocument();
    fireEvent.click(screen.getByText("acme-api"));
    expect(screen.queryByText("acme-api · claude")).not.toBeInTheDocument();
  });

  it("closes on the close button", async () => {
    const onClose = vi.fn();
    render(<TaskManagerModal onClose={onClose} />);
    await screen.findByText(/app total/);
    fireEvent.click(screen.getByRole("button", { name: "close task manager" }));
    expect(onClose).toHaveBeenCalled();
  });
});
