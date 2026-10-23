import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

const H = vi.hoisted(() => ({ RunReadinessChecks: vi.fn(), ResolveReadiness: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => H);
vi.mock("../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => () => {}) }));

import { ReadinessModal, SETTINGS_TAB_KEY } from "./ReadinessModal";

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  H.RunReadinessChecks.mockResolvedValue([
    { id: "accessibility", title: "Accessibility permission", detail: "Needed for the hotkey.", status: "fail", required: true, action: "settings-pane:Privacy_Accessibility", actionLabel: "Open System Settings" },
    { id: "github", title: "GitHub", detail: "Not connected.", status: "warn", required: false, action: "app-settings:accounts", actionLabel: "Connect" },
    { id: "whisper", title: "Whisper model", detail: "ggml-base.bin", status: "ok", required: false },
  ]);
  H.ResolveReadiness.mockResolvedValue(undefined);
});

describe("ReadinessModal", () => {
  it("lists checks with status, routes fixes to Go or to a Settings tab", async () => {
    const onClose = vi.fn();
    const openSettings = vi.fn();
    render(<ReadinessModal onClose={onClose} openSettings={openSettings} />);
    expect(await screen.findByText("1 required item missing")).toBeInTheDocument();
    expect(screen.getByText("Accessibility permission").closest(".ready-row")).toHaveAttribute("data-status", "fail");
    expect(screen.getByText("Whisper model").closest(".ready-row")).toHaveAttribute("data-status", "ok");

    fireEvent.click(screen.getByRole("button", { name: "Open System Settings" }));
    await waitFor(() => expect(H.ResolveReadiness).toHaveBeenCalledWith("settings-pane:Privacy_Accessibility"));

    fireEvent.click(screen.getByRole("button", { name: "Connect" }));
    expect(localStorage.getItem(SETTINGS_TAB_KEY)).toBe("accounts");
    expect(openSettings).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("re-checks by itself when the window regains focus", async () => {
    render(<ReadinessModal onClose={vi.fn()} openSettings={vi.fn()} />);
    await screen.findByText("GitHub");
    expect(screen.queryByRole("button", { name: /re-check/i })).toBeNull();
    H.RunReadinessChecks.mockResolvedValue([{ id: "github", title: "GitHub", detail: "Signed in as me.", status: "ok", required: false }]);
    fireEvent(window, new Event("focus"));
    expect(await screen.findByText("All required items are in place")).toBeInTheDocument();
  });
});
