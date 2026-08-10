import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const H = vi.hoisted(() => ({ DraftTodoMessage: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => H);

import { TodoActionModal } from "./TodoActionModal";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("TodoActionModal", () => {
  it("drafts on open and copies the (edited) text", async () => {
    H.DraftTodoMessage.mockResolvedValue("Please review PR o/r#42.");
    const writeText = vi.fn();
    Object.assign(navigator, { clipboard: { writeText } });

    render(
      <TodoActionModal kind="rerequest_pr" title="Ping #42" refKey="o/r#42" rationale="stale" actionLabel="Draft reviewer nudge" onClose={() => {}} />,
    );

    await waitFor(() =>
      expect(H.DraftTodoMessage).toHaveBeenCalledWith("rerequest_pr", "Ping #42", "o/r#42", "stale"),
    );
    expect(await screen.findByDisplayValue("Please review PR o/r#42.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Copy/i }));
    expect(writeText).toHaveBeenCalledWith("Please review PR o/r#42.");
  });

  it("shows an error when drafting fails", async () => {
    H.DraftTodoMessage.mockRejectedValue("boom");
    render(
      <TodoActionModal kind="make_pr" title="t" refKey="" rationale="" actionLabel="Draft PR description" onClose={() => {}} />,
    );
    expect(await screen.findByText("boom")).toBeInTheDocument();
  });
});
