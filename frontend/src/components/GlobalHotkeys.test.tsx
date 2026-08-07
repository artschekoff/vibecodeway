import { render, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const { GetVCSPRsForContext, RefreshContextPRs, BrowserOpenURL } = vi.hoisted(() => ({
  GetVCSPRsForContext: vi.fn(),
  RefreshContextPRs: vi.fn(),
  BrowserOpenURL: vi.fn(),
}));
vi.mock("../../wailsjs/go/main/App", () => ({ GetVCSPRsForContext, RefreshContextPRs }));
vi.mock("../../wailsjs/runtime/runtime", () => ({ BrowserOpenURL }));

const mockTab = vi.hoisted(() => ({ current: { id: "t1", type: "context", contextId: "c1", contextKey: "JH-42", title: "JH-42", pinned: false } as Record<string, unknown> | null }));
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({
    tabs: mockTab.current ? [mockTab.current] : [],
    activeTabId: mockTab.current ? "t1" : null,
    focusNextTab: vi.fn(),
    focusPrevTab: vi.fn(),
  }),
}));

import { GlobalHotkeys } from "./GlobalHotkeys";
import { HotkeyProvider } from "../hotkeys/HotkeyProvider";

const writeText = vi.fn();

function renderHotkeys() {
  render(
    <HotkeyProvider platform="darwin" keybindings={[]}>
      <GlobalHotkeys />
    </HotkeyProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockTab.current = { id: "t1", type: "context", contextId: "c1", contextKey: "JH-42", title: "JH-42", pinned: false };
  Object.assign(navigator, { clipboard: { writeText } });
});

describe("GlobalHotkeys", () => {
  it("⌘G then P opens the context's first PR in the browser", async () => {
    GetVCSPRsForContext.mockResolvedValue([{ url: "https://gh/pr/1" }]);
    renderHotkeys();
    fireEvent.keyDown(window, { key: "g", metaKey: true });
    fireEvent.keyDown(window, { key: "p" });
    await waitFor(() => expect(BrowserOpenURL).toHaveBeenCalledWith("https://gh/pr/1"));
  });

  it("⌘G then P no-ops when the context has no PR", async () => {
    GetVCSPRsForContext.mockResolvedValue([]);
    renderHotkeys();
    fireEvent.keyDown(window, { key: "g", metaKey: true });
    fireEvent.keyDown(window, { key: "p" });
    await waitFor(() => expect(GetVCSPRsForContext).toHaveBeenCalled());
    expect(BrowserOpenURL).not.toHaveBeenCalled();
  });

  it("⌘P then R reloads the PR list", () => {
    renderHotkeys();
    fireEvent.keyDown(window, { key: "p", metaKey: true });
    fireEvent.keyDown(window, { key: "r" });
    expect(RefreshContextPRs).toHaveBeenCalledWith("c1");
  });

  it("⌘C then N copies the context key", () => {
    renderHotkeys();
    fireEvent.keyDown(window, { key: "c", metaKey: true });
    fireEvent.keyDown(window, { key: "n" });
    expect(writeText).toHaveBeenCalledWith("JH-42");
  });

  it("a bare ⌘C does NOT preventDefault (native copy still works)", () => {
    renderHotkeys();
    const e = new KeyboardEvent("keydown", { key: "c", metaKey: true, cancelable: true, bubbles: true });
    window.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(false);
    expect(writeText).not.toHaveBeenCalled();
  });
});
