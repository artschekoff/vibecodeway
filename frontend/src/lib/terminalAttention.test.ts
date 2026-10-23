import { describe, it, expect, vi, beforeEach } from "vitest";
import { registerTermSession } from "./terminalRegistry";
import { markWaiting, clearWaiting, waitingList, jumpToNextWaiting, FOCUS_TERMINAL_EVENT, readPrefs, writePrefs } from "./terminalAttention";

beforeEach(() => {
  for (const e of waitingList()) clearWaiting(e.sessionId);
  localStorage.clear();
});

describe("terminalAttention", () => {
  it("marks sessions with their context, lists oldest first, clears", () => {
    registerTermSession("s1", { contextKey: "cc-rename", contextId: "c1", tabLabel: "api" });
    registerTermSession("s2", { contextKey: "met-investig", contextId: "c2", tabLabel: "web" });
    markWaiting("s2", "Agent is waiting", "", false);
    markWaiting("s1", "Done", "turn complete", true);
    expect(waitingList().map((e) => e.sessionId)).toEqual(["s2", "s1"]);
    expect(waitingList()[1]).toMatchObject({ contextId: "c1", contextKey: "cc-rename", tabLabel: "api", visible: true });
    clearWaiting("s2");
    expect(waitingList().map((e) => e.sessionId)).toEqual(["s1"]);
  });

  it("jumpToNextWaiting cycles and asks the owning panel to focus the session", () => {
    vi.useFakeTimers();
    registerTermSession("s1", { contextKey: "a", contextId: "c1", tabLabel: "x" });
    registerTermSession("s2", { contextKey: "", tabLabel: "ask" }); // Ask AI terminal: no context
    markWaiting("s1", "t", "", false);
    markWaiting("s2", "t", "", false);
    const tabs = { openEditorTab: vi.fn(), openAskTab: vi.fn() };
    const focused: string[] = [];
    window.addEventListener(FOCUS_TERMINAL_EVENT, (e) => focused.push((e as CustomEvent).detail.sessionId));

    expect(jumpToNextWaiting(tabs)?.sessionId).toBe("s1");
    expect(tabs.openEditorTab).toHaveBeenCalledWith("c1", "a");
    expect(jumpToNextWaiting(tabs)?.sessionId).toBe("s2");
    expect(tabs.openAskTab).toHaveBeenCalled();
    expect(jumpToNextWaiting(tabs)?.sessionId).toBe("s1");
    vi.runAllTimers();
    expect(focused).toEqual(["s1", "s2", "s1"]);
    vi.useRealTimers();
  });

  it("prefs default to on and persist", () => {
    expect(readPrefs()).toEqual({ toast: true, native: true, highlight: true });
    writePrefs({ toast: false, native: true, highlight: true });
    expect(readPrefs().toast).toBe(false);
  });
});
