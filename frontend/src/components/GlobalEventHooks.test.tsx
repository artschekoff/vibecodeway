import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const { ListGlobalEventHooks, CreateGlobalEventHook, DeleteGlobalEventHook } = vi.hoisted(() => ({
  ListGlobalEventHooks: vi.fn(),
  CreateGlobalEventHook: vi.fn(),
  DeleteGlobalEventHook: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  ListGlobalEventHooks, CreateGlobalEventHook, DeleteGlobalEventHook,
}));

import GlobalEventHooks from "./GlobalEventHooks";

beforeEach(() => {
  ListGlobalEventHooks.mockReset();
  CreateGlobalEventHook.mockReset();
  DeleteGlobalEventHook.mockReset();
});

describe("GlobalEventHooks", () => {
  it("creates a global hook via the editor", async () => {
    ListGlobalEventHooks.mockResolvedValue([]);
    CreateGlobalEventHook.mockResolvedValue({ id: "g1" });
    const user = userEvent.setup();
    render(<GlobalEventHooks />);
    await screen.findByText(/no hooks/i);

    await user.selectOptions(screen.getByTestId("hook-event"), "jira_update");
    await user.click(screen.getByRole("button", { name: /add hook/i }));

    expect(CreateGlobalEventHook).toHaveBeenCalledWith("jira_update", "os_notification", "");
  });
});
