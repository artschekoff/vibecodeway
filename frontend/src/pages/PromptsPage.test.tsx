import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const H = vi.hoisted(() => ({
  ListAIPrompts: vi.fn(),
  UpdateAIPrompt: vi.fn(),
  ResetAIPrompt: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({
  ListAIPrompts: H.ListAIPrompts,
  UpdateAIPrompt: H.UpdateAIPrompt,
  ResetAIPrompt: H.ResetAIPrompt,
}));

import PromptsPage from "./PromptsPage";

beforeEach(() => {
  vi.clearAllMocks();
  H.UpdateAIPrompt.mockResolvedValue(undefined);
  H.ResetAIPrompt.mockResolvedValue(undefined);
});

describe("PromptsPage", () => {
  it("renders a card per prompt with its label and editable body", async () => {
    H.ListAIPrompts.mockResolvedValue([
      { id: "event_extraction", label: "Event Extraction", body: "extract events", defaultBody: "extract events", placeholders: [{ name: "context_key", description: "Project key" }], updatedAt: "" },
      { id: "ask_ai", label: "Ask AI", body: "you are a helper", defaultBody: "you are a helper", placeholders: [], updatedAt: "" },
    ]);
    render(<PromptsPage />);

    expect(await screen.findByText("Event Extraction")).toBeInTheDocument();
    expect(screen.getByText("Ask AI")).toBeInTheDocument();
    expect(screen.getByDisplayValue("extract events")).toBeInTheDocument();
    expect(screen.getByDisplayValue("you are a helper")).toBeInTheDocument();
    // Placeholder chips render for prompts that declare them.
    expect(screen.getByText("{{context_key}}")).toBeInTheDocument();
  });

  it("shows the empty state when no prompts exist", async () => {
    H.ListAIPrompts.mockResolvedValue([]);
    render(<PromptsPage />);
    expect(await screen.findByText(/No prompts found/i)).toBeInTheDocument();
  });

  it("surfaces a load error instead of silently showing nothing", async () => {
    H.ListAIPrompts.mockRejectedValue(new Error("boom"));
    render(<PromptsPage />);
    await waitFor(() => expect(screen.getByText(/boom/)).toBeInTheDocument());
  });

  it("saves an edited body", async () => {
    H.ListAIPrompts.mockResolvedValue([
      { id: "ask_ai", label: "Ask AI", body: "old", defaultBody: "old", placeholders: [], updatedAt: "" },
    ]);
    render(<PromptsPage />);

    const ta = await screen.findByDisplayValue("old");
    fireEvent.change(ta, { target: { value: "new" } });
    fireEvent.click(screen.getByRole("button", { name: /Save/i }));
    await waitFor(() => expect(H.UpdateAIPrompt).toHaveBeenCalledWith("ask_ai", "new"));
  });
});
