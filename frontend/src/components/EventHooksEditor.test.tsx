import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import EventHooksEditor from "./EventHooksEditor";

describe("EventHooksEditor", () => {
  it("lists hooks and calls onAdd with the selected values", async () => {
    const onAdd = vi.fn();
    const onDelete = vi.fn();
    render(
      <EventHooksEditor
        title="Event Hooks"
        hooks={[{ id: "h1", eventCategory: "pr_update", actionType: "os_notification", scriptCommand: "" }]}
        onAdd={onAdd}
        onDelete={onDelete}
      />,
    );
    expect(screen.getByText(/GitHub PR update → OS notification/)).toBeInTheDocument();

    const user = userEvent.setup();
    await user.selectOptions(screen.getByTestId("hook-event"), "code_review");
    await user.selectOptions(screen.getByTestId("hook-action"), "run_script");
    await user.type(screen.getByTestId("hook-script"), "echo $CONTEXT_KEY");
    await user.click(screen.getByRole("button", { name: /add hook/i }));

    expect(onAdd).toHaveBeenCalledWith("code_review", "run_script", "echo $CONTEXT_KEY");
  });
});
