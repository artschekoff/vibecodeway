import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { UpsertContextText, OpenPlanInTerminal } = vi.hoisted(() => ({
  UpsertContextText: vi.fn(),
  OpenPlanInTerminal: vi.fn(),
}));
vi.mock("../../../wailsjs/go/main/App", () => ({
  UpsertContextText: UpsertContextText.mockResolvedValue(undefined),
  OpenPlanInTerminal: OpenPlanInTerminal.mockResolvedValue(undefined),
}));

import { PlanModal } from "../PlanModal";

const plan = { id: "p1", contextId: "c1", body: "# Plan\nX" } as any;

beforeEach(() => {
  vi.clearAllMocks();
  UpsertContextText.mockResolvedValue(undefined);
  OpenPlanInTerminal.mockResolvedValue(undefined);
});

describe("PlanModal", () => {
  it("renders the plan body", () => {
    render(<PlanModal plan={plan} contextId="c1" onClose={vi.fn()} />);
    expect(screen.getByText(/# Plan/)).toBeInTheDocument();
    expect(screen.getByText(/X/)).toBeInTheDocument();
  });

  it("copies the plan body to the clipboard", async () => {
    const user = userEvent.setup();
    // Spy on clipboard after userEvent.setup() has initialized it (mirrors
    // the pattern in PersonAssignmentDetail.test.tsx / WorkspaceMenu.test.tsx).
    const writeTextSpy = vi.spyOn(navigator.clipboard, "writeText");
    render(<PlanModal plan={plan} contextId="c1" onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /copy/i }));

    expect(writeTextSpy).toHaveBeenCalledWith("# Plan\nX");
  });

  it("saves the plan to the context", async () => {
    const user = userEvent.setup();
    render(<PlanModal plan={plan} contextId="c1" onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /save to context/i }));

    expect(UpsertContextText).toHaveBeenCalledWith("", "c1", expect.any(String), "# Plan\nX");
  });

  it("opens the plan in the terminal", async () => {
    const user = userEvent.setup();
    render(<PlanModal plan={plan} contextId="c1" onClose={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /open in terminal/i }));

    expect(OpenPlanInTerminal).toHaveBeenCalledWith("c1", "p1");
  });
});
