import type { ComponentProps } from "react";
import { render, screen, waitFor, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { ReadRepoFile, SetCodeTourStep, SetCodeTourDecision, StageFile, DiscardFile, ConfirmAction, AskTourQuestion } = vi.hoisted(() => ({
  ReadRepoFile: vi.fn(),
  SetCodeTourStep: vi.fn(),
  SetCodeTourDecision: vi.fn(),
  StageFile: vi.fn(),
  DiscardFile: vi.fn(),
  ConfirmAction: vi.fn(),
  AskTourQuestion: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  ReadRepoFile, SetCodeTourStep, SetCodeTourDecision, StageFile, DiscardFile, ConfirmAction, AskTourQuestion,
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
  AskTourQuestion.mockResolvedValue("");
  // Default: anchor found in current content.
  ReadRepoFile.mockResolvedValue("before\nfunc computeDashboard() {\nafter\n");
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
});

function renderCard(overrides: Partial<ComponentProps<typeof TourCard>> = {}) {
  return render(
    <TourCard
      tour={baseTour}
      repoName="acme/api"
      onClose={vi.fn()}
      onOpenFile={vi.fn()}
      onLocateAnchor={vi.fn()}
      onAskAuthorAgent={vi.fn()}
      {...overrides}
    />
  );
}

describe("TourCard", () => {
  it("renders step m/n and What/Why for the current step", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
    expect(screen.getByText("Step 1 of 2")).toBeInTheDocument();
    expect(screen.getByText("Add analytics dashboard")).toBeInTheDocument();
    expect(screen.getByText("Added the compute entry point")).toBeInTheDocument();
    expect(screen.getByText("Keeps aggregation out of the HTTP layer")).toBeInTheDocument();
    await waitFor(() => expect(ReadRepoFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
  });

  it("opens the step's file and locates the anchor once content resolves", async () => {
    const onOpenFile = vi.fn();
    const onLocateAnchor = vi.fn();
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={onOpenFile} onLocateAnchor={onLocateAnchor} onAskAuthorAgent={vi.fn()} />);
    await waitFor(() => expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/a.ts", "modified", "head"));
    await waitFor(() => expect(onLocateAnchor).toHaveBeenCalledWith("/r/api", "src/a.ts", "func computeDashboard("));
  });

  it("navigates forward/back, persisting the step via SetCodeTourStep, and disables nav at the ends", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
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
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("stage step"));
    await waitFor(() => expect(StageFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    await waitFor(() => expect(SetCodeTourDecision).toHaveBeenCalledWith("tour-1", 0, "staged"));
    expect(await screen.findByText("Staged")).toBeInTheDocument();
  });

  it("discards after confirmation and records the 'discarded' decision", async () => {
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("discard step"));
    await waitFor(() => expect(ConfirmAction).toHaveBeenCalled());
    await waitFor(() => expect(DiscardFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    await waitFor(() => expect(SetCodeTourDecision).toHaveBeenCalledWith("tour-1", 0, "discarded"));
    expect(await screen.findByText("Discarded")).toBeInTheDocument();
  });

  it("does not record a decision and shows an error when StageFile rejects", async () => {
    StageFile.mockRejectedValue(new Error("staging failed"));
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("stage step"));
    await waitFor(() => expect(StageFile).toHaveBeenCalledWith("/r/api", "src/a.ts"));
    expect(await screen.findByText("staging failed")).toBeInTheDocument();
    expect(SetCodeTourDecision).not.toHaveBeenCalled();
    expect(screen.queryByText("Staged")).not.toBeInTheDocument();
  });

  it("does not record a decision and shows an error when DiscardFile rejects", async () => {
    DiscardFile.mockRejectedValue(new Error("discard failed"));
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
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
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
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
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={onOpenFile} onLocateAnchor={onLocateAnchor} onAskAuthorAgent={vi.fn()} />);
    expect(await screen.findByText("Anchor lost")).toBeInTheDocument();
    expect(onOpenFile).toHaveBeenCalledWith("/r/api", "acme/api", "src/a.ts", "modified", "head");
    expect(onLocateAnchor).not.toHaveBeenCalled();
  });

  it("marks a step broken and disables stage/discard/edit when the file can't be read, leaving nav (skip) active", async () => {
    ReadRepoFile.mockRejectedValue(new Error("no such file"));
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
    expect(await screen.findByText("File missing — step broken")).toBeInTheDocument();
    expect(screen.getByLabelText("stage step")).toBeDisabled();
    expect(screen.getByLabelText("discard step")).toBeDisabled();
    expect(screen.getByLabelText("edit step")).toBeDisabled();
    // Navigation remains available so the user can skip past the broken step.
    expect(screen.getByLabelText("next step")).not.toBeDisabled();
  });

  it("closes via the close button", async () => {
    const onClose = vi.fn();
    render(<TourCard tour={baseTour} repoName="acme/api" onClose={onClose} onOpenFile={vi.fn()} onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />);
    await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("close tour"));
    expect(onClose).toHaveBeenCalled();
  });

  describe("Q&A", () => {
    it("submits a question via the button, calling AskTourQuestion with tour id + step idx, and renders the answer", async () => {
      AskTourQuestion.mockResolvedValue("It keeps the HTTP layer thin.");
      renderCard();
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "Why here?" } });
      fireEvent.click(screen.getByLabelText("submit question"));
      await waitFor(() =>
        expect(AskTourQuestion).toHaveBeenCalledWith("tour-1", "c1", "/r/api", 0, "Why here?"));
      expect(await screen.findByText("Why here?")).toBeInTheDocument();
      expect(await screen.findByText("It keeps the HTTP layer thin.")).toBeInTheDocument();
    });

    it("submits a question via Enter in the input", async () => {
      AskTourQuestion.mockResolvedValue("Answer via enter.");
      renderCard();
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
      const input = screen.getByLabelText("tour question");
      fireEvent.change(input, { target: { value: "Enter question?" } });
      fireEvent.keyDown(input, { key: "Enter" });
      await waitFor(() =>
        expect(AskTourQuestion).toHaveBeenCalledWith("tour-1", "c1", "/r/api", 0, "Enter question?"));
      expect(await screen.findByText("Answer via enter.")).toBeInTheDocument();
    });

    it("shows the card's inline error affordance when AskTourQuestion rejects", async () => {
      AskTourQuestion.mockRejectedValue(new Error("ask failed"));
      renderCard();
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "Why?" } });
      fireEvent.click(screen.getByLabelText("submit question"));
      const err = await screen.findByText("ask failed");
      expect(err).toBeInTheDocument();
      expect(err.getAttribute("role")).toBe("alert");
    });

    it("accumulates history across two questions", async () => {
      AskTourQuestion.mockResolvedValueOnce("First answer").mockResolvedValueOnce("Second answer");
      renderCard();
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());

      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "First question" } });
      fireEvent.click(screen.getByLabelText("submit question"));
      await screen.findByText("First answer");

      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "Second question" } });
      fireEvent.click(screen.getByLabelText("submit question"));
      await screen.findByText("Second answer");

      expect(screen.getByText("First question")).toBeInTheDocument();
      expect(screen.getByText("First answer")).toBeInTheDocument();
      expect(screen.getByText("Second question")).toBeInTheDocument();
      expect(screen.getByText("Second answer")).toBeInTheDocument();
    });

    it("does not leak a stale tour's answer into a different tour's history when the tour changes mid-flight", async () => {
      // TourCard isn't guaranteed to remount on a tour swap (EditorWorkspace
      // mounts it with key={tour.id} as defense in depth, but this exercises
      // TourCard's own guard directly via rerender with the same instance).
      let resolveAsk: (answer: string) => void = () => {};
      AskTourQuestion.mockImplementation(
        () => new Promise<string>((resolve) => { resolveAsk = resolve; })
      );
      const { rerender } = renderCard();
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());

      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "Question for A" } });
      fireEvent.click(screen.getByLabelText("submit question"));
      await waitFor(() =>
        expect(AskTourQuestion).toHaveBeenCalledWith("tour-1", "c1", "/r/api", 0, "Question for A"));

      // Swap to a different tour on the same TourCard instance while the
      // question for tour A is still in flight.
      const tourB: CodeTour = { ...baseTour, id: "tour-2" };
      rerender(
        <TourCard tour={tourB} repoName="acme/api" onClose={vi.fn()} onOpenFile={vi.fn()}
          onLocateAnchor={vi.fn()} onAskAuthorAgent={vi.fn()} />
      );
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());

      // Now the stale promise for tour A resolves.
      await act(async () => {
        resolveAsk("Answer for A (stale)");
        await Promise.resolve();
        await Promise.resolve();
      });

      expect(screen.queryByText("Question for A")).not.toBeInTheDocument();
      expect(screen.queryByText("Answer for A (stale)")).not.toBeInTheDocument();

      // The stale resolution must not leave tour B's "asking" spinner stuck
      // on — typing a fresh question for B enables submit again.
      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "Question for B" } });
      expect(screen.getByLabelText("submit question")).not.toBeDisabled();
    });

    it("invokes onAskAuthorAgent with a message containing the typed question", async () => {
      const onAskAuthorAgent = vi.fn();
      renderCard({ onAskAuthorAgent });
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
      fireEvent.change(screen.getByLabelText("tour question"), { target: { value: "What does this do?" } });
      fireEvent.click(screen.getByLabelText("ask the author agent"));
      expect(onAskAuthorAgent).toHaveBeenCalledTimes(1);
      const text = onAskAuthorAgent.mock.calls[0][0] as string;
      expect(text).toContain("What does this do?");
      expect(text).toContain("src/a.ts");
      expect(AskTourQuestion).not.toHaveBeenCalled();
    });

    it("falls back to the current step's What when the question input is empty", async () => {
      const onAskAuthorAgent = vi.fn();
      renderCard({ onAskAuthorAgent });
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
      fireEvent.click(screen.getByLabelText("ask the author agent"));
      expect(onAskAuthorAgent).toHaveBeenCalledWith(
        "Question about src/a.ts: Added the compute entry point"
      );
    });
  });

  describe("agent-rule snippet", () => {
    it("copies the rule text to the clipboard, referencing .vibecodeway/tour.json", async () => {
      renderCard();
      await waitFor(() => expect(screen.getByText("Step 1 of 2")).toBeInTheDocument());
      fireEvent.click(screen.getByLabelText("copy agent rule"));
      expect(navigator.clipboard.writeText).toHaveBeenCalledTimes(1);
      const text = (navigator.clipboard.writeText as ReturnType<typeof vi.fn>).mock.calls[0][0] as string;
      expect(text).toContain(".vibecodeway/tour.json");
    });
  });
});
