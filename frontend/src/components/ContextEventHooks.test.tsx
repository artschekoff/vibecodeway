import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { ListContextEventHooks, CreateContextEventHook, DeleteContextEventHook } = vi.hoisted(() => ({
  ListContextEventHooks: vi.fn(),
  CreateContextEventHook: vi.fn(),
  DeleteContextEventHook: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  ListContextEventHooks,
  CreateContextEventHook,
  DeleteContextEventHook,
}));

import ContextEventHooks from "./ContextEventHooks";

beforeEach(() => {
  ListContextEventHooks.mockReset();
  CreateContextEventHook.mockReset();
  DeleteContextEventHook.mockReset();
});

describe("ContextEventHooks", () => {
  it("lists existing hooks", async () => {
    ListContextEventHooks.mockResolvedValue([
      { id: "h1", contextId: "c1", eventCategory: "pr_update", actionType: "os_notification", scriptCommand: "", enabled: true, createdAt: "", updatedAt: "" },
    ]);
    render(<ContextEventHooks contextId="c1" />);
    const row = await screen.findByText((_, el) => el?.textContent === "GitHub PR update → OS notification");
    expect(row).toBeInTheDocument();
  });

  it("creates a run_script hook via the add form", async () => {
    ListContextEventHooks.mockResolvedValue([]);
    CreateContextEventHook.mockResolvedValue({ id: "h2" });
    const user = userEvent.setup();
    render(<ContextEventHooks contextId="c1" />);
    await screen.findByText(/no hooks/i);

    await user.selectOptions(screen.getByTestId("hook-event"), "code_review");
    await user.selectOptions(screen.getByTestId("hook-action"), "run_script");
    await user.type(screen.getByTestId("hook-script"), "notify-send $CONTEXT_KEY");
    await user.click(screen.getByRole("button", { name: /add hook/i }));

    expect(CreateContextEventHook).toHaveBeenCalledWith("c1", "code_review", "run_script", "notify-send $CONTEXT_KEY");
  });
});
