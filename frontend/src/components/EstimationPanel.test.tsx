import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect } from "vitest";
import EstimationPanel from "./EstimationPanel";
import type { ContextSnapshot, EstimationLog } from "../types";

function makeSnapshot(overrides: Partial<ContextSnapshot> = {}): ContextSnapshot {
  return {
    contextId: "ctx-1",
    currentStepDesc: "",
    currentStepConfidence: "medium",
    currentStepFrom: "[]",
    currentStepOverridden: false,
    sectionOperational: "",
    sectionNarrative: "",
    sectionEngineering: "",
    status: "in_progress",
    aiNextStepsShort: "",
    aiNextStepsMid: "",
    aiNextStepsLong: "",
    userNextStepsShort: "",
    userNextStepsMid: "",
    userNextStepsLong: "",
    isStale: false,
    version: 1,
    estimationConfidence: "medium",
    estimationBasis: "2 requirements at ~2h each",
    ...overrides,
  };
}

function makeLog(overrides: Partial<EstimationLog> = {}): EstimationLog {
  return {
    id: "log-1",
    contextId: "ctx-1",
    recordedAt: "2026-05-12T10:00:00Z",
    triggerType: "initial",
    triggerDescription: "first analysis",
    estimatedHoursAfter: 4,
    estimationBasis: "2 requirements at ~2h each",
    ...overrides,
  };
}

describe("EstimationPanel", () => {
  it("shows 'No estimate yet' when snapshot has no estimatedHours", () => {
    render(<EstimationPanel snapshot={makeSnapshot()} history={[]} />);
    expect(screen.getByText(/no estimate yet/i)).toBeInTheDocument();
  });

  it("renders estimated hours", () => {
    render(
      <EstimationPanel
        snapshot={makeSnapshot({ estimatedHours: 3.5 })}
        history={[]}
      />
    );
    expect(screen.getByText(/3\.5\s*h/i)).toBeInTheDocument();
  });

  it("renders completion date when present", () => {
    render(
      <EstimationPanel
        snapshot={makeSnapshot({
          estimatedHours: 2,
          estimatedCompletionDate: "2026-05-12T14:00:00Z",
        })}
        history={[]}
      />
    );
    expect(screen.getByText(/May 12/i)).toBeInTheDocument();
  });

  it("renders estimation basis text", () => {
    render(
      <EstimationPanel
        snapshot={makeSnapshot({ estimatedHours: 4, estimationBasis: "3 open requirements" })}
        history={[]}
      />
    );
    expect(screen.getByText(/3 open requirements/i)).toBeInTheDocument();
  });

  it("renders confidence badge", () => {
    render(
      <EstimationPanel
        snapshot={makeSnapshot({ estimatedHours: 4, estimationConfidence: "high" })}
        history={[]}
      />
    );
    expect(screen.getByText(/high/i)).toBeInTheDocument();
  });

  it("does not render history toggle when history is empty", () => {
    render(
      <EstimationPanel snapshot={makeSnapshot({ estimatedHours: 2 })} history={[]} />
    );
    expect(screen.queryByRole("button", { name: /history/i })).not.toBeInTheDocument();
  });

  it("renders history toggle when log entries exist", () => {
    const history = [makeLog()];
    render(
      <EstimationPanel snapshot={makeSnapshot({ estimatedHours: 4 })} history={history} />
    );
    expect(screen.getByRole("button", { name: /history/i })).toBeInTheDocument();
  });

  it("expands history on toggle button click", async () => {
    const user = userEvent.setup();
    const history = [
      makeLog({ id: "log-2", triggerType: "re-analysis", deltaHours: 2, estimatedHoursBefore: 2, estimatedHoursAfter: 4 }),
      makeLog({ id: "log-1", triggerType: "initial", estimatedHoursAfter: 2 }),
    ];
    render(
      <EstimationPanel snapshot={makeSnapshot({ estimatedHours: 4 })} history={history} />
    );

    await user.click(screen.getByRole("button", { name: /history/i }));
    expect(screen.getByText(/\+2\.0h/)).toBeInTheDocument();
    expect(screen.getByText(/initial/i)).toBeInTheDocument();
  });
});
