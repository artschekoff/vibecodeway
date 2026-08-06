import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, waitFor } from "@testing-library/react";

// Mock Wails bindings used by App.tsx to resolve platform + keybindings.
const H = vi.hoisted(() => ({
  GetKeybindings: vi.fn(),
  ListAllContexts: vi.fn(),
  ListOrganizations: vi.fn(),
}));
vi.mock("../wailsjs/go/main/App", () => H);

const R = vi.hoisted(() => ({
  Environment: vi.fn(),
  EventsOn: vi.fn(() => vi.fn()),
  EventsOff: vi.fn(),
  SendNotification: vi.fn(),
  InitializeNotifications: vi.fn(),
}));
vi.mock("../wailsjs/runtime/runtime", () => R);

import { HotkeyProvider, useHotkey } from "./hotkeys/HotkeyProvider";
import App from "./App";

function HelpProbe({ fn }: { fn: () => void }) {
  useHotkey("help.open", fn);
  return null;
}

describe("App hotkeys mounting", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    H.GetKeybindings.mockResolvedValue([]);
    H.ListAllContexts.mockResolvedValue([]);
    H.ListOrganizations.mockResolvedValue([]);
    R.Environment.mockResolvedValue({ platform: "darwin", buildType: "dev", arch: "arm64" });
  });

  it("renders the app shell without crashing once bindings/platform resolve", async () => {
    const { container } = render(<App />);
    await waitFor(() => expect(H.GetKeybindings).toHaveBeenCalled());
    expect(container.querySelector(".app-shell")).toBeTruthy();
  });

  it("dispatches a hotkey through a HotkeyProvider mounted at the root (darwin: mod+h)", async () => {
    const fn = vi.fn();
    render(
      <HotkeyProvider platform="darwin" keybindings={[]}>
        <HelpProbe fn={fn} />
      </HotkeyProvider>
    );
    fireEvent.keyDown(window, { key: "h", metaKey: true });
    await waitFor(() => expect(fn).toHaveBeenCalledTimes(1));
  });

  it("dispatches a hotkey through a HotkeyProvider mounted at the root (other: ctrl+h)", async () => {
    const fn = vi.fn();
    render(
      <HotkeyProvider platform="other" keybindings={[]}>
        <HelpProbe fn={fn} />
      </HotkeyProvider>
    );
    fireEvent.keyDown(window, { key: "h", ctrlKey: true });
    await waitFor(() => expect(fn).toHaveBeenCalledTimes(1));
  });
});
