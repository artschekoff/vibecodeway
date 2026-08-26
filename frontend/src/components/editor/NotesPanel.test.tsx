import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { ListEvents, AddUserEvent, DeleteUserEvent } = vi.hoisted(() => ({
  ListEvents: vi.fn(),
  AddUserEvent: vi.fn(),
  DeleteUserEvent: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({ ListEvents, AddUserEvent, DeleteUserEvent }));

import { NotesPanel } from "./NotesPanel";

const evt = (id: string, type: string, summary: string) => ({
  id, contextId: "ctx1", type, timestamp: "2026-08-20T14:32:00Z",
  sourceType: "user", sourceId: "", author: "me", summary, rawReference: "", createdAt: "",
});

/** The panel starts collapsed; the composer only exists once it is opened. */
const openComposer = () => fireEvent.click(screen.getByLabelText("open notes"));

beforeEach(() => { vi.clearAllMocks(); });

describe("NotesPanel", () => {
  it("collapses to the latest note, hiding older ones and the composer", async () => {
    ListEvents.mockResolvedValue([
      evt("e1", "user_note", "older note"),
      evt("e3", "user_note", "newer note"),
    ]);
    render(<NotesPanel contextId="ctx1" />);

    await waitFor(() => expect(screen.getByText("newer note")).toBeInTheDocument());
    expect(screen.queryByText("older note")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("note text")).not.toBeInTheDocument();
  });

  it("prompts in the collapsed block when the context has no notes", async () => {
    ListEvents.mockResolvedValue([]);
    render(<NotesPanel contextId="ctx1" />);

    await waitFor(() => expect(screen.getByText("What did you just do here?")).toBeInTheDocument());
  });

  it("opens the composer on click and shows the whole log", async () => {
    ListEvents.mockResolvedValue([
      evt("e1", "user_note", "older note"),
      evt("e3", "user_note", "newer note"),
    ]);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(screen.getByText("newer note")).toBeInTheDocument());

    openComposer();

    expect(screen.getByLabelText("note text")).toBeInTheDocument();
    const shown = screen.getAllByText(/^(newer|older) note$/).map((el) => el.textContent);
    expect(shown).toEqual(["newer note", "older note"]);
  });

  it("lists only user_note events, newest first", async () => {
    ListEvents.mockResolvedValue([
      evt("e1", "user_note", "older note"),
      evt("e2", "decision_made", "not a note"),
      evt("e3", "user_note", "newer note"),
    ]);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(screen.getByText("newer note")).toBeInTheDocument());
    openComposer();

    expect(screen.queryByText("not a note")).not.toBeInTheDocument();
    const shown = screen.getAllByText(/^(newer|older) note$/).map((el) => el.textContent);
    expect(shown).toEqual(["newer note", "older note"]);
  });

  it("shows an empty state in the open log when the context has no notes", async () => {
    ListEvents.mockResolvedValue([]);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    expect(screen.getByText("No notes yet.")).toBeInTheDocument();
  });

  it("closes without saving", async () => {
    ListEvents.mockResolvedValue([]);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    fireEvent.click(screen.getByLabelText("close notes"));

    expect(screen.queryByLabelText("note text")).not.toBeInTheDocument();
    expect(AddUserEvent).not.toHaveBeenCalled();
  });

  it("appends a note as a user_note event, clears the draft and collapses back", async () => {
    ListEvents.mockResolvedValue([]);
    AddUserEvent.mockResolvedValue(evt("e9", "user_note", "did the migration"));
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    fireEvent.change(screen.getByLabelText("note text"), { target: { value: "  did the migration  " } });
    ListEvents.mockResolvedValue([evt("e9", "user_note", "did the migration")]);
    fireEvent.click(screen.getByLabelText("add note"));

    await waitFor(() =>
      expect(AddUserEvent).toHaveBeenCalledWith("ctx1", "user_note", "did the migration"));
    // Collapsed again, showing the note that was just written.
    await waitFor(() => expect(screen.queryByLabelText("note text")).not.toBeInTheDocument());
    expect(screen.getByText("did the migration")).toBeInTheDocument();

    // Re-opening starts from a clean draft.
    openComposer();
    expect((screen.getByLabelText("note text") as HTMLTextAreaElement).value).toBe("");
  });

  it("submits on cmd+enter", async () => {
    ListEvents.mockResolvedValue([]);
    AddUserEvent.mockResolvedValue(evt("e9", "user_note", "quick"));
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    const box = screen.getByLabelText("note text");
    fireEvent.change(box, { target: { value: "quick" } });
    fireEvent.keyDown(box, { key: "Enter", metaKey: true });

    await waitFor(() => expect(AddUserEvent).toHaveBeenCalledWith("ctx1", "user_note", "quick"));
  });

  it("collapses on escape", async () => {
    ListEvents.mockResolvedValue([]);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    fireEvent.keyDown(screen.getByLabelText("note text"), { key: "Escape" });

    expect(screen.queryByLabelText("note text")).not.toBeInTheDocument();
    expect(AddUserEvent).not.toHaveBeenCalled();
  });

  it("does not add an empty note", async () => {
    ListEvents.mockResolvedValue([]);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    fireEvent.change(screen.getByLabelText("note text"), { target: { value: "   " } });
    fireEvent.keyDown(screen.getByLabelText("note text"), { key: "Enter", metaKey: true });

    expect(AddUserEvent).not.toHaveBeenCalled();
  });

  it("deletes a note and reloads", async () => {
    ListEvents.mockResolvedValue([evt("e1", "user_note", "stale note")]);
    DeleteUserEvent.mockResolvedValue(undefined);
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(screen.getByText("stale note")).toBeInTheDocument());
    openComposer();

    ListEvents.mockResolvedValue([]);
    fireEvent.click(screen.getByLabelText("delete note e1"));

    await waitFor(() => expect(DeleteUserEvent).toHaveBeenCalledWith("ctx1", "e1"));
    await waitFor(() => expect(screen.getByText("No notes yet.")).toBeInTheDocument());
  });

  it("keeps the draft and stays open when saving fails", async () => {
    ListEvents.mockResolvedValue([]);
    AddUserEvent.mockRejectedValue(new Error("offline"));
    render(<NotesPanel contextId="ctx1" />);
    await waitFor(() => expect(ListEvents).toHaveBeenCalled());
    openComposer();

    const box = screen.getByLabelText("note text") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "unsaved" } });
    fireEvent.click(screen.getByLabelText("add note"));

    await waitFor(() => expect(AddUserEvent).toHaveBeenCalled());
    expect(box.value).toBe("unsaved");
  });
});
