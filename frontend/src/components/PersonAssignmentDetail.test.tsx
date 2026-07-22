import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PersonAssignmentDetail } from "./PersonAssignmentDetail";
import type { PersonAssignment, PersonAssignmentVersion, ReasoningChainStep } from "../types/index";

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

  it("renders no checklist checkboxes when body has no task lines", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /add step/i })).not.toBeInTheDocument();
  });
});

describe("PersonAssignmentDetail — interactive checklist", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders a checkbox per task line and text lines as plain text", () => {
    const pa = makePA({ body: "note\n- [ ] do A\n- [x] do B" });
    render(<PersonAssignmentDetail pa={pa} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByText("note")).toBeInTheDocument();
    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes).toHaveLength(2);
    expect(checkboxes[0]).not.toBeChecked();
    expect(checkboxes[1]).toBeChecked();
    expect(screen.getByText("do A")).toBeInTheDocument();
    expect(screen.getByText("do B")).toBeInTheDocument();
  });

  it("toggling a checkbox persists the toggled body via SaveAssignmentOutput", async () => {
    const pa = makePA({ body: "- [ ] a\n- [ ] b" });
    (App.SaveAssignmentOutput as ReturnType<typeof vi.fn>).mockResolvedValue(
      makePA({ body: "- [ ] a\n- [x] b" })
    );
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={pa} steps={[makeStep()]} onUpdated={onUpdated} />);
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]);
    await waitFor(() =>
      expect(App.SaveAssignmentOutput).toHaveBeenCalledWith("pa-1", "- [ ] a\n- [x] b")
    );
    expect(onUpdated).toHaveBeenCalled();
  });

  it("Add step button appends a new empty step via SaveAssignmentOutput", async () => {
    const pa = makePA({ body: "- [x] a" });
    (App.SaveAssignmentOutput as ReturnType<typeof vi.fn>).mockResolvedValue(
      makePA({ body: "- [x] a\n- [ ] " })
    );
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={pa} steps={[makeStep()]} onUpdated={onUpdated} />);
    fireEvent.click(screen.getByRole("button", { name: /add step/i }));
    await waitFor(() =>
      expect(App.SaveAssignmentOutput).toHaveBeenCalledWith("pa-1", "- [x] a\n- [ ] ")
    );
    expect(onUpdated).toHaveBeenCalled();
  });

  it("toggling a checkbox after an unsaved textarea edit persists both the edit and the toggle (no data loss)", async () => {
    const pa = makePA({ body: "- [ ] a\n- [ ] b" });
    (App.SaveAssignmentOutput as ReturnType<typeof vi.fn>).mockResolvedValue(
      makePA({ body: "- [ ] a extra text\n- [x] b" })
    );
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={pa} steps={[makeStep()]} onUpdated={onUpdated} />);

    // User types in the textarea, diverging outputDraft from pa.body (unsaved).
    const textarea = screen.getByRole("textbox", { name: /output/i });
    fireEvent.change(textarea, { target: { value: "- [ ] a extra text\n- [ ] b" } });

    // Then clicks a checkbox — this must persist the draft's edit AND the toggle together.
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]);

    await waitFor(() =>
      expect(App.SaveAssignmentOutput).toHaveBeenCalledWith(
        "pa-1",
        "- [ ] a extra text\n- [x] b"
      )
    );
    expect(onUpdated).toHaveBeenCalled();
  });
});

describe("PersonAssignmentDetail — inline path:line chips", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders an inline code span as a clickable chip separate from surrounding text", () => {
    const pa = makePA({ body: "- [ ] fix `auth.go:42` now" });
    render(<PersonAssignmentDetail pa={pa} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByRole("button", { name: "auth.go:42" })).toBeInTheDocument();
    expect(screen.getByText("fix")).toBeInTheDocument();
    expect(screen.getByText("now")).toBeInTheDocument();
  });

  it("clicking the chip copies its value to the clipboard without toggling the checkbox", async () => {
    const user = userEvent.setup();
    const writeTextSpy = vi.spyOn(navigator.clipboard, "writeText");
    const pa = makePA({ body: "- [ ] fix `auth.go:42` now" });
    render(<PersonAssignmentDetail pa={pa} steps={[makeStep()]} onUpdated={vi.fn()} />);

    const chip = screen.getByRole("button", { name: "auth.go:42" });
    await user.click(chip);

    expect(writeTextSpy).toHaveBeenCalledWith("auth.go:42");
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    expect(App.SaveAssignmentOutput).not.toHaveBeenCalled();
  });
});

describe("PersonAssignmentDetail — prompt edit + modal", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Edit button appears when llmPrompt exists", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByRole("button", { name: /edit/i })).toBeInTheDocument();
  });

  it("Edit button not present when llmPrompt is absent", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep({ llmPrompt: undefined })]} onUpdated={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /edit/i })).not.toBeInTheDocument();
  });

  it("clicking Edit shows prompt textarea with current prompt", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    const ta = screen.getByRole("textbox", { name: /prompt/i });
    expect((ta as HTMLTextAreaElement).value).toBe("You are an analyst. Generate for Alice.");
  });

  it("Send button calls RunAssignmentStepWithCustomPrompt and shows modal", async () => {
    (App.RunAssignmentStepWithCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue("Preview output from LLM");
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(await screen.findByText("Preview output from LLM")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /confirm/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /cancel preview/i })).toBeInTheDocument();
  });

  it("Confirm saves version and calls onUpdated", async () => {
    (App.RunAssignmentStepWithCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue("Preview body");
    (App.CommitAssignmentVersionFromCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue(makePA({ body: "Preview body" }));
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={onUpdated} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    await screen.findByText("Preview body");
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));
    await waitFor(() => expect(App.CommitAssignmentVersionFromCustomPrompt).toHaveBeenCalledWith(
      "pa-1", 0, "You are an analyst. Generate for Alice.", "Preview body"
    ));
    expect(onUpdated).toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("Cancel on modal closes without saving", async () => {
    (App.RunAssignmentStepWithCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue("Preview body");
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    await screen.findByRole("dialog");
    fireEvent.click(screen.getByRole("button", { name: /cancel preview/i }));
    expect(App.CommitAssignmentVersionFromCustomPrompt).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("Cancel in edit mode returns to view without calling API", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    expect(screen.getByRole("textbox", { name: /prompt/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /cancel prompt edit/i }));
    expect(screen.queryByRole("textbox", { name: /prompt/i })).not.toBeInTheDocument();
    expect(App.RunAssignmentStepWithCustomPrompt).not.toHaveBeenCalled();
  });
});

const makeVersion = (overrides: Partial<PersonAssignmentVersion> = {}): PersonAssignmentVersion => ({
  id: "v-1",
  personAssignmentId: "pa-1",
  body: "Old assignment body",
  isActive: false,
  createdAt: "2024-01-01T00:00:00Z",
  ...overrides,
});

describe("version selector", () => {
  beforeEach(() => vi.clearAllMocks());

  it("selector not rendered when no versions exist", async () => {
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    await waitFor(() => expect(App.ListAssignmentVersions).toHaveBeenCalledWith("pa-1"));
    expect(screen.queryByRole("combobox", { name: /versions/i })).not.toBeInTheDocument();
  });

  it("selector renders when versions exist", async () => {
    const versions = [
      makeVersion({ id: "v-1", createdAt: "2024-01-01T00:00:00Z" }),
      makeVersion({ id: "v-2", createdAt: "2024-01-02T00:00:00Z" }),
    ];
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue(versions);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole("combobox", { name: /versions/i })).toBeInTheDocument());
  });

  it("selecting a version opens diff modal", async () => {
    const versions = [
      makeVersion({ id: "v-1", body: "Old body", createdAt: "2024-01-01T00:00:00Z" }),
    ];
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue(versions);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole("combobox", { name: /versions/i })).toBeInTheDocument());
    fireEvent.change(screen.getByRole("combobox", { name: /versions/i }), { target: { value: "v-1" } });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("confirm calls SetActiveAssignmentVersion and closes modal", async () => {
    const versions = [
      makeVersion({ id: "v-1", body: "Old body", createdAt: "2024-01-01T00:00:00Z" }),
    ];
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue(versions);
    const updatedPA = makePA({ body: "Old body" });
    (App.SetActiveAssignmentVersion as ReturnType<typeof vi.fn>).mockResolvedValue(updatedPA);
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={onUpdated} />);
    await waitFor(() => expect(screen.getByRole("combobox", { name: /versions/i })).toBeInTheDocument());
    fireEvent.change(screen.getByRole("combobox", { name: /versions/i }), { target: { value: "v-1" } });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));
    await waitFor(() => expect(App.SetActiveAssignmentVersion).toHaveBeenCalledWith("pa-1", "v-1"));
    expect(onUpdated).toHaveBeenCalledWith(updatedPA);
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("cancel closes modal without calling SetActiveAssignmentVersion", async () => {
    const versions = [
      makeVersion({ id: "v-1", body: "Old body", createdAt: "2024-01-01T00:00:00Z" }),
    ];
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue(versions);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    await waitFor(() => expect(screen.getByRole("combobox", { name: /versions/i })).toBeInTheDocument());
    fireEvent.change(screen.getByRole("combobox", { name: /versions/i }), { target: { value: "v-1" } });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /cancel version restore/i }));
    expect(App.SetActiveAssignmentVersion).not.toHaveBeenCalled();
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("shows error message when ListAssignmentVersions rejects", async () => {
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockRejectedValue(new Error("network failure"));
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByRole("alert")).toBeInTheDocument()
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/failed to load versions/i);
    expect(screen.queryByRole("combobox", { name: /versions/i })).not.toBeInTheDocument();
  });
});
