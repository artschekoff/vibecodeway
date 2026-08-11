import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetAppLogs, ClearAppLogs } = vi.hoisted(() => ({
  GetAppLogs: vi.fn(),
  ClearAppLogs: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetAppLogs, ClearAppLogs }));

import AppLogsPage from "./AppLogsPage";

const infoEntry = {
  id: 1, level: "info", ts: 1.5, caller: "openai/client.go:200",
  msg: "llm.chat_completion model=gpt-4o tokens=10→5 (total 15) in 2.1s",
  createdAt: "2026-08-11T00:00:00Z",
  fields: JSON.stringify({ request: '[{"role":"user","content":"hi"}]' }),
};

beforeEach(() => {
  vi.clearAllMocks();
  GetAppLogs.mockResolvedValue([infoEntry]);
  ClearAppLogs.mockResolvedValue(undefined);
});

describe("AppLogsPage", () => {
  it("offers an Info level filter option", async () => {
    render(<AppLogsPage />);
    expect(await screen.findByRole("option", { name: "Info" })).toBeInTheDocument();
  });

  it("opens the detail modal with the full request when a row is clicked", async () => {
    render(<AppLogsPage />);
    const user = userEvent.setup();
    await user.click(await screen.findByText(/llm.chat_completion/));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText(/"role":"user"/)).toBeInTheDocument();
  });
});
