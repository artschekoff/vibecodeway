import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PersonAssignmentDetail } from "./PersonAssignmentDetail";
import type { PersonAssignment, ReasoningChainStep } from "../types/index";

vi.mock("../../wailsjs/go/main/App", () => ({
  SaveAssignmentOutput: vi.fn(),
  RunAssignmentStepWithCustomPrompt: vi.fn(),
  CommitAssignmentVersionFromCustomPrompt: vi.fn(),
  ListAssignmentVersions: vi.fn().mockResolvedValue([]),
  SetActiveAssignmentVersion: vi.fn(),
}));

import * as App from "../../wailsjs/go/main/App";

const makePA = (overrides: Partial<PersonAssignment> = {}): PersonAssignment => ({
  id: "pa-1",
  contextId: "ctx-1",
  name: "Alice",
  body: "Original assignment body",
  isOutdated: false,
  updatedAt: new Date().toISOString(),
  ...overrides,
});

const makeStep = (overrides: Partial<ReasoningChainStep> = {}): ReasoningChainStep => ({
  id: "step-1",
  entityType: "person_assignment",
  entityId: "pa-1",
  stepIndex: 0,
  sourceType: "initial",
  messageContent: "Alice context from Jira",
  inputState: "",
  outputState: "Original assignment body",
  compressedSummary: "",
  eventsSnapshot: "",
  promptVersion: "pa_initial_v1",
  isCurrent: true,
  createdAt: new Date().toISOString(),
  llmPrompt: "You are an analyst. Generate for Alice.",
  ...overrides,
});

describe("PersonAssignmentDetail", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shows SOURCE, PROMPT, and OUTPUT section labels", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/source/i)).toBeInTheDocument();
    expect(screen.getByText(/prompt/i)).toBeInTheDocument();
    expect(screen.getByText(/output/i)).toBeInTheDocument();
  });

  it("SOURCE section shows step messageContent", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByText("Alice context from Jira")).toBeInTheDocument();
  });

  it("output textarea contains current body", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    const textarea = screen.getByRole("textbox", { name: /output/i });
    expect((textarea as HTMLTextAreaElement).value).toBe("Original assignment body");
  });

  it("Save button calls SaveAssignmentOutput with edited body", async () => {
    (App.SaveAssignmentOutput as ReturnType<typeof vi.fn>).mockResolvedValue(makePA({ body: "Edited body" }));
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={onUpdated} />);

    const textarea = screen.getByRole("textbox", { name: /output/i });
    fireEvent.change(textarea, { target: { value: "Edited body" } });
    fireEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(App.SaveAssignmentOutput).toHaveBeenCalledWith("pa-1", "Edited body"));
    expect(onUpdated).toHaveBeenCalled();
  });

  it("displays truncated llmPrompt in PROMPT section", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/You are an analyst/)).toBeInTheDocument();
  });

  it("shows 'No prompt stored' when llmPrompt is absent", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep({ llmPrompt: undefined })]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/no prompt stored/i)).toBeInTheDocument();
  });

  it("Save button is disabled when body is unchanged", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });

  it("Save button becomes enabled after editing", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    const textarea = screen.getByRole("textbox", { name: /output/i });
    fireEvent.change(textarea, { target: { value: "New content" } });
    expect(screen.getByRole("button", { name: /save/i })).not.toBeDisabled();
  });

  it("renders fallback message when steps is empty", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/no reasoning steps available/i)).toBeInTheDocument();
  });
});
