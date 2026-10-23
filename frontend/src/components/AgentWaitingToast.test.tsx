import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";

const tabs = vi.hoisted(() => ({ openEditorTab: vi.fn(), openAskTab: vi.fn() }));
vi.mock("../contexts/TabContext", () => ({ useTabContext: () => tabs }));

import { AgentWaitingToast } from "./AgentWaitingToast";
import { registerTermSession } from "../lib/terminalRegistry";
import { markWaiting, clearWaiting, waitingList, FOCUS_TERMINAL_EVENT } from "../lib/terminalAttention";

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  for (const e of waitingList()) clearWaiting(e.sessionId);
});

describe("AgentWaitingToast", () => {
  it("shows hidden waiting terminals only, jumps on click, dismisses on ×", () => {
    vi.useFakeTimers();
    registerTermSession("s1", { contextKey: "cc-rename", contextId: "c1", tabLabel: "acme-api" });
    registerTermSession("s2", { contextKey: "visible-one", contextId: "c2", tabLabel: "web" });
    render(<AgentWaitingToast />);
    expect(screen.queryByRole("status")).toBeNull();

    act(() => { markWaiting("s1", "Agent is waiting for input", "", false); });
    act(() => { markWaiting("s2", "Agent is waiting for input", "", true); });
    expect(screen.getByText("cc-rename")).toBeInTheDocument();
    expect(screen.queryByText("visible-one")).toBeNull();

    const focused: string[] = [];
    window.addEventListener(FOCUS_TERMINAL_EVENT, (e) => focused.push((e as CustomEvent).detail.sessionId));
    fireEvent.click(screen.getByTitle("Jump to this terminal"));
    expect(tabs.openEditorTab).toHaveBeenCalledWith("c1", "cc-rename");
    act(() => { vi.runAllTimers(); });
    expect(focused).toEqual(["s1"]);

    fireEvent.click(screen.getByRole("button", { name: "Dismiss cc-rename" }));
    expect(screen.queryByRole("status")).toBeNull();
    vi.useRealTimers();
  });
});
