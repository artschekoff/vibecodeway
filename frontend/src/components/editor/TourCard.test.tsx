import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { ReadRepoFile, SetCodeTourStep, SetCodeTourDecision, StageFile, DiscardFile, ConfirmAction } = vi.hoisted(() => ({
  ReadRepoFile: vi.fn(),
  SetCodeTourStep: vi.fn(),
  SetCodeTourDecision: vi.fn(),
  StageFile: vi.fn(),
  DiscardFile: vi.fn(),
  ConfirmAction: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  ReadRepoFile, SetCodeTourStep, SetCodeTourDecision, StageFile, DiscardFile, ConfirmAction,
}));

import { TourCard } from "./TourCard";
import type { CodeTour } from "../../types";

const baseTour: CodeTour = {
  id: "tour-1",
  contextId: "c1",
  repoPath: "/r/api",
  source: "generated",
  title: "Add analytics dashboard",
  steps: [
    { file: "src/a.ts", anchor: "func computeDashboard(", what: "Added the compute entry point", why: "Keeps aggregation out of the HTTP layer" },
    { file: "src/b.ts", anchor: "const B = 1", what: "Second change", why: "Second reason" },
  ],
  stepIndex: 0,
  decisions: {},
  createdAt: "2026-09-11T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
  SetCodeTourStep.mockResolvedValue(undefined);
  SetCodeTourDecision.mockResolvedValue(undefined);
  StageFile.mockResolvedValue(undefined);
  DiscardFile.mockResolvedValue(undefined);
  ConfirmAction.mockResolvedValue(true);
  // Default: anchor found in current content.
  ReadRepoFile.mockResolvedValue("before\nfunc computeDashboard() {\nafter\n");
});

describe("TourCard", () => {
  it("renders step m/n and What/Why for the current step", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    expect(screen.getByText("Step 1 of 2")).toBeInTheDocument();
    expect(screen.getByText("Add analytics dashboard")).toBeInTheDocument();
    expect(screen.getByText("Added the compute entry point")).toBeInTheDocument();
    expect(screen.getByText("Keeps aggregation out of the HTTP layer")).toBeInTheDocument();
    await waitFor(() => expect(ReadRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
  });

  it("opens the step's file and locates the anchor once content resolves", async () => {
    const onOpenFile = vi.fn();
    const onLocateAnchor = vi.fn();
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={onOpenFile} onLocateAnchor={onLocateAnchor} />);
    await waitFor(() => expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/a.ts", "modified", "head"));
    await waitFor(() => expect(onLocateAnchor).toHaveBeenCalledWith("/r/api", "src/a.ts", "func computeDashboard("));
  });

  it("navigates forward/back, persisting the step via SetCodeTourStep, and disables nav at the ends", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    expect(screen.getByLabelText("previous step")).toBeDisabled();
    expect(screen.getByLabelText("next step")).not.toBeDisabled();

    fireEvent.click(screen.getByLabelText("next step"));
    await waitFor(() => expect(SetCodeTourStep).toHaveBeenCalledWith("tour-1", 1));
    await waitFor(() => expect(screen.getByText("Step 2 of 2")).toBeInTheDocument());
    expect(screen.getByLabelText("next step")).toBeDisabled();

    fireEvent.click(screen.getByLabelText("previous step"));
    await waitFor(() => expect(SetCodeTourStep).toHaveBeenCalledWith("tour-1", 0));
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
  });

  it("stages the current step's file and records the 'staged' decision", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("stage step"));
    await waitFor(() => expect(StageFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    await waitFor(() => expect(SetCodeTourDecision).toHaveBeenCalledWith("tour-1", 0, "staged"));
    expect(await screen.findByText("Staged")).toBeInTheDocument();
  });

  it("discards after confirmation and records the 'discarded' decision", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard step"));
    await waitFor(() => expect(ConfirmAction).toHaveBeenCalled());
    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    await waitFor(() => expect(SetCodeTourDecision).toHaveBeenCalledWith("tour-1", 0, "discarded"));
    expect(await screen.findByText("Discarded")).toBeInTheDocument();
  });

  it("does not record a decision and shows an error when StageFile rejects", async () => {
    StageFile.mockRejectedValue(new Error("staging failed"));
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("stage step"));
    await waitFor(() => expect(StageFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    expect(await screen.findByText("staging failed")).toBeInTheDocument();
    expect(SetCodeTourDecision).not.toHaveBeenCalled();
    expect(screen.queryByText("Staged")).not.toBeInTheDocument();
  });

  it("does not record a decision and shows an error when DiscardFile rejects", async () => {
    DiscardFile.mockRejectedValue(new Error("discard failed"));
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard step"));
    await waitFor(() => expect(ConfirmAction).toHaveBeenCalled());
    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    expect(await screen.findByText("discard failed")).toBeInTheDocument();
    expect(SetCodeTourDecision).not.toHaveBeenCalled();
    expect(screen.queryByText("Discarded")).not.toBeInTheDocument();
  });

  it("does not discard when confirmation is declined", async () => {
    ConfirmAction.mockResolvedValue(false);
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard step"));
    await waitFor(() => expect(ConfirmAction).toHaveBeenCalled());
    expect(DiscardFile).not.toHaveBeenCalled();
    expect(SetCodeTourDecision).not.toHaveBeenCalled();
  });

  it("shows an 'anchor lost' badge when the anchor is not found in current content, but still opens the file", async () => {
    ReadRepoFile.mockResolvedValue("completely different content\n");
    const onOpenFile = vi.fn();
    const onLocateAnchor = vi.fn();
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={onOpenFile} onLocateAnchor={onLocateAnchor} />);
    expect(await screen.findByText("Anchor lost")).toBeInTheDocument();
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/a.ts", "modified", "head");
    expect(onLocateAnchor).not.toHaveBeenCalled();
  });

  it("marks a step broken and disables stage/discard/edit when the file can't be read, leaving nav (skip) active", async () => {
    ReadRepoFile.mockRejectedValue(new Error("no such file"));
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    expect(await screen.findByText("File missing — step broken")).toBeInTheDocument();
    expect(screen.getByLabelText("stage step")).toBeDisabled();
    expect(screen.getByLabelText("discard step")).toBeDisabled();
    expect(screen.getByLabelText("edit step")).toBeDisabled();
    // Navigation remains available so the user can skip past the broken step.
    expect(screen.getByLabelText("next step")).not.toBeDisabled();
  });

  it("closes via the close button", async () => {
    const onClose = vi.fn();
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={onClose} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("close tour"));
    expect(onClose).toHaveBeenCalled();
  });
});
