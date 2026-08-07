import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

const H = vi.hoisted(() => ({ SearchContexts: vi.fn() }));
vi.mock("../../wailsjs/go/main/App", () => H);

const openContextTab = vi.fn();
vi.mock("../contexts/TabContext", () => ({
  useTabContext: () => ({ openContextTab }),
}));

import { HotkeyProvider } from "../hotkeys/HotkeyProvider";
import { GlobalSearchModal } from "./GlobalSearchModal";

function renderModal() {
  return render(
    <HotkeyProvider platform="darwin" keybindings={[]}>
      <GlobalSearchModal />
    </HotkeyProvider>
  );
}

describe("GlobalSearchModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("opens on Cmd+F, lists cross-org hits with org tag, arrows + Enter open the context", async () => {
    H.SearchContexts.mockResolvedValue([
      { id: "c1", key: "JH-1", orgId: "o1", orgName: "jiro", summary: "fix login" },
      { id: "c2", key: "AB-9", orgId: "o2", orgName: "acme", summary: "billing" },
    ]);
    renderModal();
    fireEvent.keyDown(window, { key: "f", metaKey: true });
    fireEvent.change(await screen.findByPlaceholderText(/search/i), { target: { value: "x" } });
    expect(await screen.findByText("jiro")).toBeInTheDocument(); // org tag on left
    fireEvent.keyDown(window, { key: "ArrowDown" }); // highlight row 2
    fireEvent.keyDown(window, { key: "Enter" });
    expect(openContextTab).toHaveBeenCalledWith("c2", "AB-9", "acme", "o2");
  });

  it("closes on Escape", async () => {
    H.SearchContexts.mockResolvedValue([]);
    renderModal();
    fireEvent.keyDown(window, { key: "f", metaKey: true });
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
